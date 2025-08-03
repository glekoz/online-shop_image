package grpc

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"net/http"

	"github.com/glekoz/online-shop_image/internal/models"
	protoimage "github.com/glekoz/online-shop_proto/protoimage"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AppAPI interface {
	CreateEntity(ctx context.Context, service, entityID string, maxCount int) error
	DeleteEntity(ctx context.Context, service, entityID string) error
	InitialSave(ctx context.Context, service, entityID string, isCover bool, img image.Image) (string, error)
	DeleteImage(ctx context.Context, imagePath string) error
	GetEntityState(ctx context.Context, service, entityID string) (models.EntityState, error)
	SetBusyStatus(ctx context.Context, service, entityID string) (bool, error)
	SetFreeStatus(ctx context.Context, service, entityID string) (bool, error)
	GetCoverImage(ctx context.Context, service, entityID string) (string, error)
	GetImageList(ctx context.Context, service, entityID string) ([]string, error)
}

func (s *ImageServer) CreateEntity(ctx context.Context, req *protoimage.CreateEntityRequest) (*protoimage.BoolResponse, error) {
	type RequestData struct {
		Service  string `validate:"required"`
		EntityID string `validate:"required"`
		MaxCount int    `validate:"gt=0"`
	}
	var reqData RequestData
	reqData.Service = req.GetCommonMetadata().GetService()
	reqData.EntityID = req.GetCommonMetadata().GetEntityId()
	reqData.MaxCount = int(req.GetMaxCount())

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(reqData)
	if err != nil {
		return &protoimage.BoolResponse{Ok: false}, err
	}

	err = s.App.CreateEntity(ctx, reqData.Service, reqData.EntityID, reqData.MaxCount)
	if err != nil {
		return &protoimage.BoolResponse{Ok: false}, err
	}
	return &protoimage.BoolResponse{Ok: true}, nil
}

func (s *ImageServer) DeleteEntity() error {
	s.App.SetBusyStatus() // BOOL возвращается, чтобы настроить grpc retry
	s.App.DeleteEntity()
	return nil
}

// ИСПОЛЬЗУЕТСЯ ТОЛЬКО ПЕРЕД ДОБАВЛЕНИЕМ НОВЫХ ФОТОГРАФИЙ
func (s *ImageServer) SetBusyStatus() error {
	s.App.SetBusyStatus()
	return nil
}

// Первым должно приходить сообщение о метаданных

// Такой вот костыль - перед добавлением новой фотографии надо залочить (SetBusyStatus)
// в шлюзе из-за того, что фотография обрабатывается асинхронно.
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
				return status.Error(codes.InvalidArgument, "image chunk")
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

	imageID, err := s.App.InitialSave(stream.Context(), reqData.Service, reqData.EntityID, reqData.IsCover, i)
	if err != nil {
		return stream.SendAndClose(&protoimage.UploadResult{ImageId: ""})
	}
	return stream.SendAndClose(&protoimage.UploadResult{ImageId: imageID})
}

func (s *ImageServer) DeleteImage() error { // используется также при обновлении обложки
	s.App.SetBusyStatus(ctx, service, entityID) // BOOL возвращается, чтобы настроить grpc retry
	s.App.DeleteImage(ctx, imagePath)
	s.App.SetFreeStatus(ctx, service, entityID) // BOOL возвращается, чтобы настроить grpc retry
	return nil
}

func (s *ImageServer) GetCoverImage() (string, error) {
	return "", nil
}

func (s *ImageServer) GetImageList() ([]string, error) {
	return nil, nil
}
