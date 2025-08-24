package application

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"path/filepath"

	"github.com/glekoz/online-shop_image/internal/models"
	"github.com/google/uuid"
)

const (
	ImageStatusBusy = "busy"
	ImageStatusFree = "free"
)

type StorageAPI interface {
	Save(ctx context.Context, service, entityID, imageID string, img image.Image) (string, error)
	Delete(path string) error
	DeleteAll(service, entityID string) error
	GetRawImage(imagePath string) (image.Image, error)
	// UpdateMainPhoto(dir, id string, img image.Image) error - ЭТО НАДО СДЕЛАТЬ
	//ItemsInDir(dir string) (int, error)
}

type DBAPI interface {
	CreateEntity(ctx context.Context, service, entityID, status string, maxCount int) error
	DeleteEntity(ctx context.Context, service, entityID string) error
	AddImage(ctx context.Context, image models.EntityImage) error
	DeleteImage(ctx context.Context, service, entityID, imagePath string) error
	//SetCountAndFreeStatus(ctx context.Context, service, entityID, status string, images int) error
	GetEntityState(ctx context.Context, service, entityID string) (models.EntityState, error)
	SetStatus(ctx context.Context, service, entityID, status string) error
	GetCoverImage(ctx context.Context, service, entityID string) (models.EntityImage, error)
	GetImageList(ctx context.Context, service, entityID string) ([]models.EntityImage, error)
}

type AMTAPI interface {
	Publish(context.Context, []byte) error
}

type App struct {
	DB       DBAPI
	Storage  StorageAPI
	ImageAMT AMTAPI
	SC       *SyncController
	// Logger говорят, надо саму ошибку в месте появления логировать
	// Jaeger tracer
}

// метрики в хендлере, а не в сервисе

// ВОТ ТУТ УЖЕ ВСЕ ОШИБКИ НАДО ЛОГИРОВАТЬ!!!

// общение с различными сервисами уже в main функции можно настроить с помощью одного соединения amt.Dial()
// но настройки у всех разные, поэтому надо 3 экземпляра и передать
func NewApp(db DBAPI, s StorageAPI, image AMTAPI) *App {
	syncController := NewSyncController(db, s)
	return &App{DB: db, Storage: s, ImageAMT: image, SC: syncController}
}

func (a *App) CreateEntity(ctx context.Context, service, entityID string, maxCount int) error {
	loc := "App.CreateEntity"
	if err := a.DB.CreateEntity(ctx, service, entityID, ImageStatusFree, maxCount); err != nil {
		//if errors.Is(err, models.ErrUniqueViolation) {
		// как-нибудь залогировать по-особенному - В ХЕНДЛЕРЕ
		//	return models.NewError(loc, service+" "+entityID, err)
		//}
		return models.NewError(loc, service+" "+entityID, err)
	}
	return nil
}

func (a *App) DeleteEntity(ctx context.Context, service, entityID string) error {
	loc := "App.DeleteEntity"
	err := a.Storage.DeleteAll(service, entityID)
	if err != nil {
		return models.NewError(loc, service+" "+entityID, err)
	}
	err = a.DB.DeleteEntity(ctx, service, entityID)
	if err != nil {
		return models.NewError(loc, service+" "+entityID, err)
	}
	return nil
}

// этот метод вызывается из gRPC
// в шлюзе в методе, ответственном за создание записи товара, также генерируется само имя папки,
// которое передается в дальнейших запросах к этому сервису
// создается в сервисе ещё и таблица со списиком изображений,
// и таблица с количеством изображений, статусом, есть ли сейчас изображения в обработке, и общем количестве разрешенных иозбражений
func (a *App) InitialSave(ctx context.Context, service, entityID string, isCover bool, img image.Image) (string, error) { // может, сразу изображение давать? 100% зря логику вызывать не буду
	loc := "App.InitialSave"

	type Result struct {
		imageID string
		err     error
	}

	resChan := make(chan Result, 1)

	go func(ch chan<- Result) {
		defer close(ch)

		if ctx.Err() != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID, ctx.Err())}
			return
		}

		imageID := uuid.New().String()
		tmpEntityID := filepath.Join(entityID, "tmp")
		tmpImgPath, err := a.Storage.Save(ctx, service, tmpEntityID, imageID, img)
		if err != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID+" "+imageID, err)} // ок для логирования, но для передачи ошибок выше надо что-то другое придумать
			return
		}

		defer func() {
			if ctx.Err() != nil || err != nil {
				a.Storage.Delete(tmpImgPath) // удалить временное изображение, если не удалось опубликовать сообщение
				//log
			}
		}()

		if ctx.Err() != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID, ctx.Err())}
			return
		}

		amtMsg := models.ProcessImageMessage{
			Service:      service,
			EntityID:     entityID,
			ImageID:      imageID,
			IsCover:      isCover,
			TmpImagePath: tmpImgPath,
		}
		msg, err := json.Marshal(amtMsg)
		if err != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID+" "+imageID, err)}
			return
		}

		err = a.ImageAMT.Publish(ctx, msg)
		if err != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID+" "+imageID, err)}
			return
		}

		if ctx.Err() != nil {
			ch <- Result{"", models.NewError(loc, service+" "+entityID, ctx.Err())}
			return
		}
		serviceDirName := filepath.Join(service, entityID)
		a.SC.ReqCountIncrement(serviceDirName)
		ch <- Result{imageID, nil}
	}(resChan)

	select {
	case <-ctx.Done():
		// залогировать
		return "", models.NewError(loc, service+" "+entityID, ctx.Err())
	case res := <-resChan:
		if res.err != nil {
			// залогировать
			return "", res.err // хотя само по себе ерр мб нил, но логировать тогда что?
		}
		return res.imageID, nil
	}
	// И ТУТ МНЕ НЕ НУЖНА УНИКАЛЬНАЯ БЛОКИРОВКА НА ДИРЕКТОРИЮ - У МЕНЯ ТОЛЬКО 2 ОБЩИЕ МАПЫ, КОТОРЫЕ СЧИТАЮТ, НЕ ПРЕВЫШЕН ЛИ ЛИМИТ
	// И ПОТОМ ДЕЛАЙ ЧТО ХОЧЕШЬ, ТОЛЬКО ПУТЬ К ВРЕМЕННОМУ НЕОБРАБОТАННОМУ ИЗОБРАЖЕНИЮ ПЕРЕДАЙ
	// А КЭШ ДЛЯ ШЛЮЗА НУЖЕН плюс там в БД склада добавлю колонку для общего количества изображений для выбранного товара
	//
	//
	//

	// каждая загружаемая фотография сначала должна увеличивать счетчик в кэше

	// сначала проверить количество и сохранить, потом инкрементировать
	// хотя лучше сначала проверить, потом инкрементировать, а в случае ошибки декрементировать и сохранить на диск

	// так как между созданием товара и обработкой с сохранением на диск изображений есть время, за которое можно отправить миллиард фотографий
	// по команде добавления новых фоток, то надо проверять количество запросов! сумму количества запросов и изображений на диске
	// видимо, эту сумму придется хранить в кэше, чтобы шлюз тоже имел доступ к этой информации
	// и ходить при каждом запросе в БД - то ещё расточительство, организую новую мапу в памяти

	// тут будет проверка на количество фоток в директории, эта же проверка должна быть и в таблице, связанной с товаром
	// и не позволит отправить ещё фотографии уже в шлюзе
}

// а этот из AMT - уже там настраивается параллельность
// значит, нужна система ошибок и контексты
func (a *App) ProcessedSave(ctx context.Context, service, entityID, imageID, tmpImagePath string, isCover bool) error { // img = full path to temp raw image file

	loc := "App.ProcessedSave"
	errChan := make(chan error, 1)

	go func(ch chan<- error) {
		//var imagePath string
		defer close(ch)

		img, err := a.Storage.GetRawImage(tmpImagePath)
		if err != nil {
			ch <- models.NewError(loc, tmpImagePath, err)
			return
		}

		if ctx.Err() != nil {
			ch <- models.NewError(loc, "context", ctx.Err())
			return
		}

		// блок кода для обработки изображений - пока просто перекрасить в серый
		grayImg, err := toGrayScale(ctx, img)
		if err != nil {
			ch <- models.NewError(loc, tmpImagePath, err)
			return
		}

		serviceDirName := filepath.Join(service, entityID)
		token := a.SC.DirSyncChannel(serviceDirName)
		token <- struct{}{}
		var isOK bool
		defer func() {
			if !isOK {
				<-token
			}
			err := a.SC.SyncMemoryClean(ctx, serviceDirName) // сделать именованную ошибку, чтобы ещё ошибку при публикации можно было зарегистрировать
			// так эта ошибка даже нигде не читается, так что просто ЗАЛОГИРОВАТЬ
			if err != nil {
				ch <- models.NewError(loc, serviceDirName, err)
			}
		}()

		if ctx.Err() != nil {
			ch <- models.NewError(loc, "context", ctx.Err())
			return
		}

		imagePath, err := a.Storage.Save(ctx, service, entityID, imageID, grayImg)
		if err != nil {
			ch <- models.NewError(loc, service+" "+entityID+" "+imageID, err)
			return
		}
		<-token
		isOK = true
		// не надо удалять временные изображения в случае ошибки
		/*
			defer func() {
				if ctx.Err() != nil || err != nil {
					a.Storage.Delete(imagePath)
				}
			}()
		*/

		// ТУТ ДОБАВЛЯЕТСЯ ИНФОРМАЦИЯ О ПУТИ К ИЗОБРАЖЕНИЮ В СООТВ. ТАБЛИЦУ СЕРВИСА

		err = a.DB.AddImage(ctx, models.EntityImage{Service: service, EntityID: entityID, ImagePath: imagePath, IsCover: isCover})
		if err != nil {
			ch <- models.NewError(loc, imagePath, err)
			// DoRetry
			return
		}

		a.SC.ProcessCountIncrement(serviceDirName)

		// вынесу в sync
		/*
			err = a.Storage.Delete(tmpImagePath)
			if err != nil {
				// можно вынести в sync - папку целиком удалять
				// залогировать
				// не критично, что не удалось удалить временное изображение, но лучше удалить
			}
		*/
	}(errChan)

	select {
	case <-ctx.Done():
		// залогировать
		return models.NewError(loc, service+" "+entityID+" "+imageID, ctx.Err())
	case err := <-errChan:
		if err != nil {
			// залогировать
			return err // хотя само по себе ерр мб нил, но логировать тогда что?
		}
		return nil
	}
}

func (a *App) DeleteImage(ctx context.Context, service, entityID, imagePath string) error {
	loc := "App.DeleteImage"
	err := a.Storage.Delete(imagePath)
	if err != nil {
		return models.NewError(loc, imagePath, err)
	}
	err = a.DB.DeleteImage(ctx, service, entityID, imagePath)
	if err != nil {
		return models.NewError(loc, imagePath, err)
	}
	return nil
}

func (a *App) IsStatusFree(ctx context.Context, service, entityID string) (bool, error) {
	// можно добавить КЭШ
	loc := "App.IsStatusFree"
	state, err := a.DB.GetEntityState(ctx, service, entityID)
	if err != nil {
		return false, models.NewError(loc, service+" "+entityID, err)
	}
	if state.Status == ImageStatusBusy {
		return false, nil
	}
	return true, nil
}

func (a *App) SetBusyStatus(ctx context.Context, service, entityID string) (bool, error) {
	loc := "App.SetBusyStatus"
	serviceDirName := filepath.Join(service, entityID)
	token := a.SC.DirSyncChannel(serviceDirName)
	token <- struct{}{}
	defer func() {
		<-token
	}()
	state, err := a.DB.GetEntityState(ctx, service, entityID)
	if err != nil {
		return false, models.NewError(loc, service+" "+entityID, err)
	}
	if state.Status == ImageStatusBusy {
		return false, nil
	}
	err = a.DB.SetStatus(ctx, service, entityID, ImageStatusBusy)
	if err != nil {
		return false, models.NewError(loc, service+" "+entityID, err)
	}
	return true, nil
}

func (a *App) SetFreeStatus(ctx context.Context, service, entityID string) (bool, error) {
	loc := "App.SetFreeStatus"
	serviceDirName := filepath.Join(service, entityID)
	token := a.SC.DirSyncChannel(serviceDirName)
	token <- struct{}{}
	defer func() {
		<-token
	}()
	state, err := a.DB.GetEntityState(ctx, service, entityID)
	if err != nil {
		return false, models.NewError(loc, service+" "+entityID, err)
	}
	if state.Status == ImageStatusFree {
		return false, models.NewError(loc, service+" "+entityID, errors.New("unexpected shit"))
	}
	err = a.DB.SetStatus(ctx, service, entityID, ImageStatusFree)
	if err != nil {
		return false, models.NewError(loc, service+" "+entityID, err)
	}
	return true, nil
}

func (a *App) GetCoverImage(ctx context.Context, service, entityID string) (string, error) {
	loc := "App.GetCoverImage"
	image, err := a.DB.GetCoverImage(ctx, service, entityID)
	if err != nil {
		return "", models.NewError(loc, service+" "+entityID, err)
	}
	return image.ImagePath, nil
}

func (a *App) GetImageList(ctx context.Context, service, entityID string) ([]string, error) {
	loc := "App.GetImageList"
	images, err := a.DB.GetImageList(ctx, service, entityID)
	if err != nil {
		return nil, models.NewError(loc, service+" "+entityID, err)
	}
	urls := make([]string, 0, 10)
	for _, image := range images {
		urls = append(urls, image.ImagePath)
	}
	return urls, nil
}

// где добавить и использовать методы обновления БД?
// значит, НАДО ПУБЛИКОВАТЬ СООБЩЕНИЯ, ЧТО ВСЁ ОК, А СООТВЕТСТВУЮЩИЙ СЕРВИС ПРОСЛУШИВАЕТ
// И ВЫПОЛНЯЕТ НУЖНЫЕ ДЕЙСТВИЯ
// прочитать из телеги про подтверждение из другой горутины
