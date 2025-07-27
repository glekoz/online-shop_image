package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"path/filepath"
	"strings"
	"time"

	amt "github.com/Gleb988/online-shop_amt"
	"github.com/Gleb988/online-shop_image/internal/models"
	"github.com/google/uuid"
)

type StorageAPI interface {
	Save(dir, id string, img image.Image) (string, error)
	Delete(imgPath string) error
	UpdateMainPhoto(dir, id string, img image.Image) error
	ItemsInDir(dir string) (int, error)
	GetRawImage(path string) (image.Image, error)
}

type CacheAPI interface {
	Get(string) (int, error)
	Save(string, int) error
}

type App struct {
	Storage    StorageAPI
	ProductAMT amt.AMT
	UserAMT    amt.AMT
	ImageAMT   amt.AMT
	SC         *SyncController
}

// ВОТ ТУТ УЖЕ ВСЕ ОШИБКИ НАДО ЛОГИРОВАТЬ!!!

// общение с различными сервисами уже в main функции можно настроить с помощью одного соединения amt.Dial()
// но настройки у всех разные, поэтому надо 3 экземпляра и передать
func NewApp(s StorageAPI, product, user, image amt.AMT) *App {
	syncController := NewSyncController(s, product, user)
	return &App{Storage: s, ProductAMT: product, UserAMT: user, ImageAMT: image, SC: syncController}
}

// этот метод вызывается из gRPC
// в шлюзе в методе, ответственном за создание записи товара, также генерируется само имя папки,
// которое передается в дальнейших запросах к этому сервису
// создается в сервисе ещё и таблица со списиком изображений,
// и таблица с количеством изображений, статусом, есть ли сейчас изображения в обработке, и общем количестве разрешенных иозбражений
func (a *App) InitialSave(service, dirName string, img []byte) error { // может, сразу изображение давать? 100% зря логику вызывать не буду
	serviceDirName := filepath.Join(service, dirName)
	imageReader := bytes.NewReader(img)
	imgimg, _, err := image.Decode(imageReader)
	if err != nil {
		// залогировать
		return fmt.Errorf("%w: image.Decode failed: "+err.Error(), models.ErrOperationAction)
	}

	tmpName := uuid.New().String()
	tmpDirPath := filepath.Join(serviceDirName, "tmp")
	tmpImgPath, err := a.Storage.Save(tmpDirPath, tmpName, imgimg)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}

	// убрать +".jpeg", добавить метод в StorageAPI, который будет читать (открывать)
	// временное изображение
	amtMsg := models.ProcessImageMessage{
		ServiceDirName: serviceDirName,
		ImagePath:      tmpImgPath,
	}
	msg, err := json.Marshal(amtMsg)
	if err != nil {
		a.Storage.Delete(tmpImgPath) // удалить временное изображение, если не удалось опубликовать сообщение
		// залогировать
		return fmt.Errorf("%w: "+err.Error(), models.ErrOperationAction)
	}

	err = a.ImageAMT.Publish(msg)
	if err != nil {
		a.Storage.Delete(tmpImgPath) // удалить временное изображение, если не удалось опубликовать сообщение
		// залогировать
		return fmt.Errorf("%w: "+err.Error(), models.ErrNetworkAction)
	}
	a.SC.ReqCountIncrement(serviceDirName)
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
	return nil
}

// а этот из AMT
// значит, нужна система ошибок и контексты
func (a *App) ProcessedSave(serviceDirName, imgPath string) error { // img = full path to temp raw image file
	img, err := a.Storage.GetRawImage(imgPath)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}

	// блок кода для обработки изображений - пока просто перекрасить в серый
	grayImg := toGrayScale(img)

	token := a.SC.DirSyncChannel(serviceDirName)
	token <- struct{}{}
	defer func() {
		<-token
		err := a.SC.SyncMemoryClean(serviceDirName) // сделать именованную ошибку, чтобы ещё ошибку при публикации можно было зарегистрировать
		// хотя по итогу всё равно запрещу ретраить
		if err != nil {
			// залогировать
		}
	}()

	id := uuid.New().String()
	imagePath, err := a.Storage.Save(serviceDirName, id, &grayImg)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}
	// ТУТ ДОБАВЛЯЕТСЯ ИНФОРМАЦИЯ О ПУТИ К ИЗОБРАЖЕНИЮ В СООТВ. ТАБЛИЦУ СЕРВИСА
	serviceName := strings.ToLower(filepath.SplitList(serviceDirName)[0])
	dir := filepath.SplitList(serviceDirName)[1]
	tmpmsg := models.ImageSavedMessage{
		DirName:   dir,
		ImagePath: imagePath,
	}
	msg, err := json.Marshal(tmpmsg)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOperationAction)
	}
	for i := range 3 {
		var err error
		if serviceName == "product" {
			err = a.ProductAMT.Publish(msg)
		} else {
			err = a.UserAMT.Publish(msg)
		}
		if err == nil {
			break
		}
		if i == 2 {
			a.Storage.Delete(imagePath) // удалить сохраненное изображение, если не удалось опубликовать сообщение
			return fmt.Errorf("%w: "+err.Error(), models.ErrNetworkAction)
		}
		time.Sleep(2 * time.Second) // подождать 2 секунды, чтобы не перегружать сеть
	}

	a.SC.ProcessCountIncrement(serviceDirName)

	err = a.Storage.Delete(imgPath) // не самая критичная часть
	if err != nil {
		// залогировать
		// не критично, что не удалось удалить временное изображение, но лучше удалить
	}
	return nil
}

// где добавить и использовать методы обновления БД?
// значит, НАДО ПУБЛИКОВАТЬ СООБЩЕНИЯ, ЧТО ВСЁ ОК, А СООТВЕТСТВУЮЩИЙ СЕРВИС ПРОСЛУШИВАЕТ
// И ВЫПОЛНЯЕТ НУЖНЫЕ ДЕЙСТВИЯ
// прочитать из телеги про подтверждение из другой горутины
