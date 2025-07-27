package grpc

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/Gleb988/online-shop_proto/protoimage"
	"google.golang.org/grpc"
)

// А МНЕ НУЖЕН ФЛАГ, РАЗДЕЛЯЮЩИЙ ИЗОБРАЖЕНИЯ
// НАВЕРНО ПРИ ЗАПРОСЕ НАДО ДОБАВИТЬ КОЛИЧЕСТВО ФОТОГРАФИЙ
// ДОБАВИТЬ СЕМАФОР В СЕРВЕР, ЧТОБЫ ОГРАНИЧИТЬ ЧИСЛО ОДНОВРЕМЕННО
// ОБРАБАТЫВАЕМЫХ ФОТОГРАФИЙ

const (
	maxSize        = 5 << 20
	maxMessageSize = 1 << 20
)

type AppAPI interface {
}

type ImageServer struct {
	// семафор
	app AppAPI
	protoimage.UnimplementedImageServer
}

func (s *ImageServer) UploadImage(stream grpc.ClientStreamingServer[protoimage.ImageMessage, protoimage.UploadResult]) error {
	var (
		service        string
		image          bytes.Buffer
		gotServiceName bool
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
		case msg.GetService() != "":
			if gotServiceName {
				return fmt.Errorf("service already got")
			}
			service = msg.GetService()
			gotServiceName = true

		case len(msg.GetImageChunk()) > 0:
			_, err := image.Write(msg.GetImageChunk())
			if err != nil {
				return err // хотя я и так проверяю размер файла
			}
			if len(image.Bytes()) > maxSize {
				return fmt.Errorf("image is too big")
			}
		default:
			return fmt.Errorf("unexpected arguments")
		}
	}

	/*
	   блок валидации
	       if !gotMetadata == nil {
	          return status.Error(codes.InvalidArgument, "missing product metadata")
	      }
	      if metadata.Name == "" {
	          return status.Error(codes.InvalidArgument, "product name is required")
	      }
	      if imageData.Len() == 0 {
	          return status.Error(codes.InvalidArgument, "image is required")
	      }
	*/

	return nil
}
