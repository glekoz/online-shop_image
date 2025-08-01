package grpc

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"net/http"

	protoimage "github.com/glekoz/online-shop_proto/protoimage"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
)

type AppAPI interface {
	InitialSave(ctx context.Context, service, entityID string, isCover bool, img image.Image) (string, error)
}

// Первым должно приходить сообщение о метаданных

func (s *ImageServer) UploadImage(stream grpc.ClientStreamingServer[protoimage.ImageMessage, protoimage.UploadResult]) error {
	type RequestData struct {
		Service     string `validate:"required"`
		EntityID    string `validate:"required"`
		IsCover     bool
		ImageSize   int  `validate:"gt=0"`
		GotMetadata bool `validate:"required"`
	}
	var (
		img     bytes.Buffer
		reqData = RequestData{}
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
			if reqData.GotMetadata {
				return errors.New("multiple Metadata messages")
			}
			reqData.Service = msg.GetMetadata().GetService()
			reqData.EntityID = msg.GetMetadata().GetEntityId()
			reqData.IsCover = msg.GetMetadata().GetIsCover()
			reqData.GotMetadata = true
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
	reqData.ImageSize = img.Len()

	imageBytes := img.Bytes()
	imageType := http.DetectContentType(imageBytes)
	if imageType != "image/jpeg" && imageType != "image/png" {
		return errors.New("unsupported format")
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(reqData)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(imageBytes)
	i, _, err := image.Decode(reader)
	if err != nil {
		return err
	}

	imageID, err := s.app.InitialSave(stream.Context(), reqData.Service, reqData.EntityID, reqData.IsCover, i)
	if err != nil {
		return stream.SendAndClose(&protoimage.UploadResult{ImageId: ""})
	}
	return stream.SendAndClose(&protoimage.UploadResult{ImageId: imageID})
}
