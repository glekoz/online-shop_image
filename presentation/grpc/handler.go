package grpc

import (
	"bytes"
	"errors"
	"image"
	"io"
	"net/http"

	"github.com/Gleb988/online-shop_proto/protoimage"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
)

// Первым должно приходить сообщение о метаданных

func (s *ImageServer) UploadImage(stream grpc.ClientStreamingServer[protoimage.ImageMessage, protoimage.UploadResult]) error {
	type RequestData struct {
		Service        string `validate:"required"`
		DirName        string `validate:"required"`
		ImageSize      int    `validate:"gt=0"`
		GotServiceName bool   `validate:"required"`
	}
	var (
		img     bytes.Buffer
		reqData = RequestData{ImageSize: img.Len()}
	)

	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		switch {
		case msg.GetMetadata() != nil:
			if reqData.GotServiceName {
				return errors.New("multiple Metadata messages")
			}
			reqData.Service = msg.GetMetadata().Service
			reqData.DirName = msg.GetMetadata().DirName
			reqData.GotServiceName = true
		case len(msg.GetImageChunk()) > 0:
			_, err := img.Write(msg.GetImageChunk())
			if err != nil {
				return err
			}
			if img.Len() > maxSize {
				return errors.New("image is too big")
			}
		default:
			return errors.New("unexpected arguments")
		}
	}

	imageType := http.DetectContentType(img.Bytes())
	if imageType != "image/jpeg" && imageType != "image/png" {
		return errors.New("unsupported format")
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(reqData)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(img.Bytes())
	i, _, err := image.Decode(reader)
	if err != nil {
		return err
	}

	err = s.app.InitialSave(stream.Context(), reqData.Service, reqData.DirName, i)

	return err
}
