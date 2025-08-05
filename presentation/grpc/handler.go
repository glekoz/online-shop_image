package grpc

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"net/http"
	"strings"
	"sync"

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
	IsStatusFree(ctx context.Context, service, entityID string) (bool, error)
	SetBusyStatus(ctx context.Context, service, entityID string) (bool, error)
	SetFreeStatus(ctx context.Context, service, entityID string) (bool, error)
	GetCoverImage(ctx context.Context, service, entityID string) (string, error)
	GetImageList(ctx context.Context, service, entityID string) ([]string, error)
}

func (s *ImageServer) CreateEntity(ctx context.Context, req *protoimage.CreateEntityRequest) (*protoimage.BoolResponse, error) {
	var reqData models.CreateEntityRequest
	reqData.Service = req.GetCommonMetadata().GetService()
	reqData.EntityID = req.GetCommonMetadata().GetEntityId()
	reqData.MaxCount = int(req.GetMaxCount())

	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(reqData)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 3)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}

	err = s.App.CreateEntity(ctx, reqData.Service, reqData.EntityID, reqData.MaxCount)
	if err != nil {
		// обработка ошибок на уровне приложения, чтобы тут можно было норм ошибки выдать
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.AlreadyExists, "123")
	}
	return &protoimage.BoolResponse{Ok: true}, nil
}

func (s *ImageServer) DeleteEntity(ctx context.Context, req *protoimage.CommonMetadata) (*protoimage.BoolResponse, error) {
	var cm models.CommonMetadata
	cm.Service = req.GetService()
	cm.EntityID = req.GetEntityId()
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(cm)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 2)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}
	ok, err := s.App.SetBusyStatus(ctx, cm.Service, cm.EntityID) // BOOL возвращается, чтобы настроить grpc retry
	if err != nil {
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, err.Error())
	}
	if !ok {
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.FailedPrecondition, "system is busy")
	}
	// free статус уже некуда писать
	err = s.App.DeleteEntity(ctx, cm.Service, cm.EntityID)
	if err != nil {
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, err.Error())
	}
	return &protoimage.BoolResponse{Ok: false}, nil
}

func (s *ImageServer) IsStatusFree(ctx context.Context, req *protoimage.CommonMetadata) (*protoimage.BoolResponse, error) {
	var cm models.CommonMetadata
	cm.Service = req.GetService()
	cm.EntityID = req.GetEntityId()
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(cm)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 2)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}
	ok, err := s.App.IsStatusFree(ctx, cm.Service, cm.EntityID)
	if err != nil {
		return &protoimage.BoolResponse{Ok: false}, status.Error(codes.Internal, err.Error())
	}
	return &protoimage.BoolResponse{Ok: ok}, nil
}

/*
// ИСПОЛЬЗУЕТСЯ ТОЛЬКО ПЕРЕД ДОБАВЛЕНИЕМ НОВЫХ ФОТОГРАФИЙ
func (s *ImageServer) SetBusyStatus() error {
	s.App.SetBusyStatus()
	return nil
}
*/

// ЕСТЬ ПОТЕНЦИАЛ ДЛЯ КЛЮЧЕЙ ИДЕМПОНЕНТНОСТИ -
// ВСТАВЛЯТЬ В БД ИНФОРМАЦИЮ О ЗАПРОСЕ, ПРИ КОТОРОМ СОХРАНИЛОСЬ
// ИЗОБРАЖЕНИЕ, И В СЛУЧАЕ ПРЕРЫВАНИЯ ПОТОКА ИЗОБРАЖЕНИЙ
// РЕТРАИТЬ НА КЛИЕНТЕ ЦЕЛИКОМ ВЕСЬ ПОТОК (хотя я сейчас каждое изображение отдельно
// ретраить собираюсь), А ПОТОМ НА СЕРВЕРЕ СМОТРЕТЬ, ЧТО УЖЕ БЫЛО ВСТАВЛЕНО
// Первым должно приходить сообщение о метаданных
func (s *ImageServer) UploadImage(stream grpc.BidiStreamingServer[protoimage.UploadImageRequest, protoimage.UploadImageResponse]) error {
	var (
		cm  models.CommonMetadata
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
		return status.Error(codes.FailedPrecondition, "system is busy")
	}
	defer func() {
		_, err := s.App.SetFreeStatus(stream.Context(), cm.Service, cm.EntityID)
		if err != nil {
			// залогировать?
		}
	}()

	var wg sync.WaitGroup
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
			wg.Add(1)
			go func() {
				defer wg.Done()
				imageID, err := s.App.InitialSave(stream.Context(), cm.Service, cm.EntityID, isCover, i)
				if err != nil {
					// обработка ошибок - при критических сразу отменять контекст и возвращать ошибку по всему стриму
					stream.Send(&protoimage.UploadImageResponse{ImageId: "", Err: err.Error()})
				}
				stream.Send(&protoimage.UploadImageResponse{ImageId: imageID, Err: ""})
			}()
			img = bytes.Buffer{}
		default:
			return status.Error(codes.InvalidArgument, "unexpected arguments")
		}
	}
	wg.Wait()
	return nil
}

func (s *ImageServer) DeleteImage(ctx context.Context, req *protoimage.DeleteImageRequest) (*protoimage.DeleteImageResponse, error) { // используется также при обновлении обложки

	var reqData models.DeleteImageRequest
	reqData.Service = req.GetCommonMetadata().GetService()
	reqData.EntityID = req.GetCommonMetadata().GetEntityId()
	reqData.Images = req.GetImagePath()
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(reqData)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.DeleteImageResponse{Resp: nil}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 3)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.DeleteImageResponse{Resp: nil}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}

	ok, err := s.App.SetBusyStatus(ctx, reqData.Service, reqData.EntityID)
	if err != nil {
		return &protoimage.DeleteImageResponse{Resp: nil}, status.Error(codes.Internal, err.Error()) // вот тут уже можно статусы добавить, чтобы заретриаить и попозже ещё раз попробовать
	}
	if !ok {
		return &protoimage.DeleteImageResponse{Resp: nil}, status.Error(codes.FailedPrecondition, "system is busy")
	}
	defer func() {
		_, err := s.App.SetFreeStatus(ctx, reqData.Service, reqData.EntityID)
		if err != nil {
			// залогировать?
		}
	}()

	var errs []struct {
		Image string
		Error error
	}
	for _, image := range reqData.Images {
		if err = s.App.DeleteImage(ctx, image); err != nil {
			errs = append(errs, struct {
				Image string
				Error error
			}{Image: image, Error: err})
		}
	}
	if len(errs) > 0 {
		ress := []*protoimage.UploadImageResponse{}
		for _, err := range errs {
			ress = append(ress, &protoimage.UploadImageResponse{ImageId: err.Image, Err: err.Error.Error()})
		}
		return &protoimage.DeleteImageResponse{Resp: ress}, status.Error(codes.InvalidArgument, "failed to delete")
	}

	return &protoimage.DeleteImageResponse{Resp: nil}, nil
}

func (s *ImageServer) GetCoverImage(ctx context.Context, req *protoimage.CommonMetadata) (*protoimage.GetCoverImageResponse, error) {
	var cm models.CommonMetadata
	cm.Service = req.GetService()
	cm.EntityID = req.GetEntityId()
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(cm)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.GetCoverImageResponse{CoverImagePath: ""}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 2)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.GetCoverImageResponse{CoverImagePath: ""}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}
	path, err := s.App.GetCoverImage(ctx, cm.Service, cm.EntityID)
	if err != nil {
		// обработка различных ошибок, а не только этой
		return &protoimage.GetCoverImageResponse{CoverImagePath: ""}, status.Error(codes.NotFound, "no such image")
	}
	return &protoimage.GetCoverImageResponse{CoverImagePath: path}, nil
}

func (s *ImageServer) GetImageList(ctx context.Context, req *protoimage.CommonMetadata) (*protoimage.GetImageListResponse, error) {
	var cm models.CommonMetadata
	cm.Service = req.GetService()
	cm.EntityID = req.GetEntityId()
	validate := validator.New(validator.WithRequiredStructEnabled())
	err := validate.Struct(cm)
	if err != nil {
		errs, ok := err.(validator.ValidationErrors)
		if !ok {
			return &protoimage.GetImageListResponse{ImagePath: nil}, status.Error(codes.Internal, "error validation")
		}
		fields := make([]string, 2)
		for _, err := range errs {
			fields = append(fields, err.StructField())
		}
		return &protoimage.GetImageListResponse{ImagePath: nil}, status.Error(codes.InvalidArgument, strings.Join(fields, " "))
	}
	images, err := s.App.GetImageList(ctx, cm.Service, cm.EntityID)
	if err != nil {
		// обработка различных ошибок, а не только этой
		return &protoimage.GetImageListResponse{ImagePath: nil}, status.Error(codes.NotFound, "no images")
	}
	return &protoimage.GetImageListResponse{ImagePath: images}, nil
}
