package application

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"github.com/Gleb988/online-shop_image/internal/models"
	"github.com/google/uuid"
)

type StorageAPI interface {
	Save(ctx context.Context, dir, id string, img image.Image) (string, error)
	Delete(imgPath string) error
	// UpdateMainPhoto(dir, id string, img image.Image) error - ЭТО ВООБЩЕ ЗАПАРА СЕРВИСА, А НЕ КАРТИНОК
	ItemsInDir(dir string) (int, error)
	GetRawImage(path string) (image.Image, error)
}

type CacheAPI interface {
	Get(string) (int, error)
	Save(string, int) error
}

type AMTAPI interface {
	Publish(context.Context, []byte) error
}

type App struct {
	Storage    StorageAPI
	ProductAMT AMTAPI
	UserAMT    AMTAPI
	ImageAMT   AMTAPI
	SC         *SyncController
	// Logger говорят, надо саму ошибку в месте появления логировать
	// Jaeger tracer
}

// метрики в хендлере, а не в сервисе

// ВОТ ТУТ УЖЕ ВСЕ ОШИБКИ НАДО ЛОГИРОВАТЬ!!!

// общение с различными сервисами уже в main функции можно настроить с помощью одного соединения amt.Dial()
// но настройки у всех разные, поэтому надо 3 экземпляра и передать
func NewApp(s StorageAPI, product, user, image AMTAPI) *App {
	syncController := NewSyncController(s, product, user)
	return &App{Storage: s, ProductAMT: product, UserAMT: user, ImageAMT: image, SC: syncController}
}

// этот метод вызывается из gRPC
// в шлюзе в методе, ответственном за создание записи товара, также генерируется само имя папки,
// которое передается в дальнейших запросах к этому сервису
// создается в сервисе ещё и таблица со списиком изображений,
// и таблица с количеством изображений, статусом, есть ли сейчас изображения в обработке, и общем количестве разрешенных иозбражений
func (a *App) InitialSave(ctx context.Context, service, dirName string, img image.Image) error { // может, сразу изображение давать? 100% зря логику вызывать не буду

	errChan := make(chan error, 1)

	go func(ch chan<- error) (err error) {
		defer close(ch)

		serviceDirName := filepath.Join(service, dirName)

		/*
			вынести в хендлер
				imageReader := bytes.NewReader(img)
				imgimg, _, err := image.Decode(imageReader)
				if err != nil {
					ch <- fmt.Errorf("App.InitialSave: image.Decode: %w", err) // мб стоит добавиь свои модели ошибок
					return nil
				}
		*/
		if ctx.Err() != nil {
			ch <- fmt.Errorf("App.InitialSave: context: %w", ctx.Err())
			return nil
		}

		tmpName := uuid.New().String()
		tmpDirPath := filepath.Join(serviceDirName, "tmp")
		tmpImgPath, err := a.Storage.Save(ctx, tmpDirPath, tmpName, img)
		if err != nil {
			ch <- fmt.Errorf("App.InitialSave: Storage.Save: %w", err) // ок для логирования, но для передачи ошибок выше надо что-то другое придумать
			return err
		}

		defer func() {
			if ctx.Err() != nil || err != nil {
				a.Storage.Delete(tmpImgPath) // удалить временное изображение, если не удалось опубликовать сообщение
			}
		}()

		if ctx.Err() != nil {
			ch <- fmt.Errorf("App.InitialSave: context: %w", ctx.Err())
			return
		}

		amtMsg := models.ProcessImageMessage{
			ServiceDirName: serviceDirName,
			ImagePath:      tmpImgPath,
		}
		msg, err := json.Marshal(amtMsg)
		if err != nil {
			ch <- fmt.Errorf("App.InitialSave: json.Marshal: %w", err)
			return err
		}

		err = a.ImageAMT.Publish(ctx, msg)
		if err != nil {
			ch <- fmt.Errorf("App.InitialSave: ImageAMT.Publish: %w", err)
			return err
		}

		if ctx.Err() != nil {
			ch <- fmt.Errorf("App.InitialSave: context: %w", ctx.Err())
			return
		}

		a.SC.ReqCountIncrement(serviceDirName)

		return nil
	}(errChan)

	select {
	case <-ctx.Done():
		// залогировать
		return ctx.Err()
	case err := <-errChan:
		if err != nil {
			// залогировать
			return err // хотя само по себе ерр мб нил, но логировать тогда что?
		}
		return nil
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

// а этот из AMT
// значит, нужна система ошибок и контексты
func (a *App) ProcessedSave(ctx context.Context, serviceDirName, tmpImagePath string) error { // img = full path to temp raw image file

	errChan := make(chan error, 1)

	go func(ch chan<- error) (err error) {
		var imagePath string
		defer func() {
			close(ch)
			if (ctx.Err() != nil || err != nil) && imagePath != "" {
				a.Storage.Delete(imagePath)
			}
		}()

		img, err := a.Storage.GetRawImage(tmpImagePath)
		if err != nil {
			ch <- fmt.Errorf("App.ProcessedSave: Storage.GetRawImage: %w - %w", err, models.ErrDoNotRetry)
			return err
		}

		if ctx.Err() != nil {
			ch <- fmt.Errorf("App.ProcessedSave: context: %w - %w", ctx.Err(), models.ErrDoNotRetry)
			return ctx.Err()
		}

		// блок кода для обработки изображений - пока просто перекрасить в серый
		grayImg, err := toGrayScale(ctx, img)
		if err != nil {
			ch <- fmt.Errorf("App.ProcessedSave: toGrayScale: %w - %w", err, models.ErrDoNotRetry)
			return err
		}

		token := a.SC.DirSyncChannel(serviceDirName)
		token <- struct{}{}
		defer func() {
			<-token
			err := a.SC.SyncMemoryClean(ctx, serviceDirName) // сделать именованную ошибку, чтобы ещё ошибку при публикации можно было зарегистрировать
			if err != nil {
				ch <- fmt.Errorf("App.ProcessedSave: SC.SyncMemoryClean: %w - %w", err, models.ErrDoNotRetry)
			}
		}()

		if ctx.Err() != nil {
			ch <- fmt.Errorf("App.ProcessedSave: context: %w - %w", ctx.Err(), models.ErrDoNotRetry)
			return ctx.Err()
		}

		id := uuid.New().String()
		imagePath, err = a.Storage.Save(ctx, serviceDirName, id, grayImg)
		if err != nil {
			ch <- fmt.Errorf("App.ProcessedSave: Storage.Save: %w - %w", err, models.ErrDoNotRetry)
			return err
		}

		// ТУТ ДОБАВЛЯЕТСЯ ИНФОРМАЦИЯ О ПУТИ К ИЗОБРАЖЕНИЮ В СООТВ. ТАБЛИЦУ СЕРВИСА
		pathParts := filepath.SplitList(serviceDirName)
		if len(pathParts) < 2 {
			ch <- fmt.Errorf("App.ProcessedSave: invalid serviceDirName: %w", models.ErrDoNotRetry)
			return err
		}
		serviceName := strings.ToLower(pathParts[0])
		dir := pathParts[1]
		tmpmsg := models.ImageSavedMessage{
			DirName:   dir,
			ImagePath: imagePath,
		}
		msg, err := json.Marshal(tmpmsg)
		if err != nil {
			ch <- fmt.Errorf("App.ProcessedSave: json.Marshal: %w - %w", err, models.ErrDoNotRetry)
			return err
		}

		if serviceName == "product" {
			err = a.ProductAMT.Publish(ctx, msg)
		} else {
			err = a.UserAMT.Publish(ctx, msg)
		}
		if err != nil {
			ch <- fmt.Errorf("App.ProcessedSave: ProductAMT. or UserAMT.Publish: %w - %w", err, models.ErrDoNotRetry)
			return err
		}

		a.SC.ProcessCountIncrement(serviceDirName)

		err = a.Storage.Delete(tmpImagePath) // не самая критичная часть
		if err != nil {
			// залогировать
			// не критично, что не удалось удалить временное изображение, но лучше удалить
		}
		return nil

	}(errChan)

	select {
	case <-ctx.Done():
		// залогировать
		return ctx.Err()
	case err := <-errChan:
		if err != nil {
			// залогировать
			return err // хотя само по себе ерр мб нил, но логировать тогда что?
		}
		return nil
	}
}

func (a *App) Delete(ctx context.Context, path string) error {
	err := a.Storage.Delete(path)
	if err != nil {
		return fmt.Errorf("App.Delete: Storage.Delete: %w", err)
	}
	return nil
}

// где добавить и использовать методы обновления БД?
// значит, НАДО ПУБЛИКОВАТЬ СООБЩЕНИЯ, ЧТО ВСЁ ОК, А СООТВЕТСТВУЮЩИЙ СЕРВИС ПРОСЛУШИВАЕТ
// И ВЫПОЛНЯЕТ НУЖНЫЕ ДЕЙСТВИЯ
// прочитать из телеги про подтверждение из другой горутины
