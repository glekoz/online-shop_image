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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AppAPI interface {
	CreateEntity(ctx context.Context, service, entityID string, maxCount int) error
	DeleteEntity(ctx context.Context, service, entityID string) error
	InitialSave(ctx context.Context, service, entityID string, isCover bool, img image.Image) (string, error)
	DeleteImage(ctx context.Context, imagePath string) error
	IsStatusFree(ctx context.Context, service, entityID string) (bool, error)
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

// ЕСТЬ ПОТЕНЦИАЛ ДЛЯ КЛЮЧЕЙ ИДЕМПОНЕНТНОСТИ -
// ВСТАВЛЯТЬ В БД ИНФОРМАЦИЮ О ЗАПРОСЕ, ПРИ КОТОРОМ СОХРАНИЛОСЬ
// ИЗОБРАЖЕНИЕ, И В СЛУЧАЕ ПРЕРЫВАНИЯ ПОТОКА ИЗОБРАЖЕНИЙ
// РЕТРАИТЬ НА КЛИЕНТЕ ЦЕЛИКОМ ВЕСЬ ПОТОК (хотя я сейчас каждое изображение отдельно
// ретраить собираюсь), А ПОТОМ НА СЕРВЕРЕ СМОТРЕТЬ, ЧТО УЖЕ БЫЛО ВСТАВЛЕНО
// Первым должно приходить сообщение о метаданных
func (s *ImageServer) UploadImage(stream grpc.BidiStreamingServer[protoimage.UploadImageRequest, protoimage.UploadImageResponse]) error {
	type CommonMetadata struct {
		Service  string `validate:"required"`
		EntityID string `validate:"required"`
		//GotMetadata bool   `validate:"required"`
	}
	var (
		cm  CommonMetadata
		img bytes.Buffer
	)

	msg, err := stream.Recv()
	if err != nil {
		return err // вот тут уже можно статусы добавить
	}
	cm.Service = msg.GetMetadata().GetService()
	cm.EntityID = msg.GetMetadata().GetEntityId()
	//cm.GotMetadata = true

	validate := validator.New(validator.WithRequiredStructEnabled())
	err = validate.Struct(cm)
	if err != nil {
		return err // вот тут уже можно статусы добавить
	}

	ok, err := s.App.SetBusyStatus(stream.Context(), cm.Service, cm.EntityID)
	if err != nil {
		return err // вот тут уже можно статусы добавить, чтобы заретриаить и попозже ещё раз попробовать
	}
	if !ok {
		return status.Error(codes.ResourceExhausted, "Unable to set busy status")
	}
	defer func() {
		_, err := s.App.SetFreeStatus(stream.Context(), cm.Service, cm.EntityID)
		if err != nil {
			// залогировать?
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err // вот тут уже можно статусы добавить
		}
		//
		// ОДИН РАЗ ПРОВЕРИТЬ МЕТАДАННЫЕ
		// А ПОТОМ ТОЛЬКО ФЛАГ И КАРТИНКУ ОТПРАВЛЯТЬ
		switch {
		case len(msg.GetImageChunk()) > 0:
			_, err := img.Write(msg.GetImageChunk())
			if err != nil {
				return status.Error(codes.InvalidArgument, "image chunk")
			}
			if img.Len() > maxSize {
				return errors.New("image is too big") // вот тут уже можно статусы добавить
			}
		case msg.GetIsCover() != nil:
			if img.Len() < 1 {
				return status.Error(codes.InvalidArgument, "Cover flag should be sent after image")
			}
			imageBytes := img.Bytes()
			imageType := http.DetectContentType(imageBytes)
			if imageType != "image/jpeg" && imageType != "image/png" {
				return status.Error(codes.InvalidArgument, "unsupported format")
			}

			reader := bytes.NewReader(imageBytes)
			i, _, err := image.Decode(reader)
			if err != nil {
				return err // вот тут уже можно статусы добавить
			}

			isCover := msg.GetIsCover().GetValue()
			imageID, err := s.App.InitialSave(stream.Context(), cm.Service, cm.EntityID, isCover, i)
			if err != nil {
				stream.Send(&protoimage.UploadImageResponse{ImageId: "", Err: err.Error()})
			}
			stream.Send(&protoimage.UploadImageResponse{ImageId: imageID, Err: ""})
			img = bytes.Buffer{}
		default:
			return status.Error(codes.InvalidArgument, "unexpected arguments")
		}
	}
	return nil
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
