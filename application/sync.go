package application

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Gleb988/online-shop_image/internal/models"
)

type SyncController struct {
	ProductAMT        AMTAPI
	UserAMT           AMTAPI
	ImageCount        map[string]int
	ReqCountMutex     sync.RWMutex
	ReqCount          map[string]int
	ProcessCountMutex sync.RWMutex
	ProcessCount      map[string]int
	DirSyncMutex      sync.RWMutex
	DirSync           map[string]chan struct{}
}

func NewSyncController(s StorageAPI, product, user AMTAPI) *SyncController {
	imageCount := make(map[string]int)
	reqCount := make(map[string]int)
	processCount := make(map[string]int)
	dirSync := make(map[string]chan struct{})
	return &SyncController{ProductAMT: product, UserAMT: user,
		ImageCount: imageCount, ReqCount: reqCount, ProcessCount: processCount, DirSync: dirSync}
}

/*
// уже не надо, так как в БД склада добавлю информацию, есть ли фотографии в обработке
func (sc *SyncController) PossibleToSave(service, dir string) (bool, error) { // есть потенциал использовать RWMutex, чтобы улучшить производительность
	sc.ReqCountMutex.Lock()
	sc.ReqCount[dir]++
	count, ok := sc.ImageCount[dir]
	if !ok {
		count, err := sc.Storage.ItemsInDir(service, dir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		sc.ImageCount[dir] = count
	}
	if count > 9 {
		sc.ReqCount[dir]--
		return false, errors.ErrUnsupported // переделать ошибку
	}
	total := sc.ReqCount[dir] + count
	sc.Cache.Save(dir, total) // TTL 60 sec поставлю
	sc.ReqCountMutex.Unlock()
	return true, nil
}
*/

func (sc *SyncController) ProcessCountIncrement(dir string) {
	sc.ProcessCountMutex.Lock()
	defer sc.ProcessCountMutex.Unlock()
	sc.ProcessCount[dir]++
}

func (sc *SyncController) SyncMemoryClean(ctx context.Context, dir string) error {
	sc.ProcessCountMutex.RLock()
	sc.ReqCountMutex.RLock()
	defer func() {
		sc.ReqCountMutex.RUnlock()
		sc.ProcessCountMutex.RUnlock()
	}()

	if sc.ProcessCount[dir] == sc.ReqCount[dir] {
		serviceName := strings.ToLower(filepath.SplitList(dir)[0])
		itemDir := filepath.SplitList(dir)[1]
		mod := models.CompleteImageProcessMessage{
			SavedItemDir: itemDir,
			TotalCount:   sc.ProcessCount[dir],
		}
		// ПРИ ПОЛУЧЕНИИ ЭТОГО СООБЩЕНИЯ ОБНОВЛЯЕТСЯ СТОЛБИК С КОЛИЧЕСТВОМ ИЗОБРАЖЕНИЙ В СЕРВИСЕ
		msg, err := json.Marshal(mod)
		if err != nil {
			// залогировать
			return err
		}
		if serviceName == "product" {
			err = sc.ProductAMT.Publish(ctx, msg)
		} else {
			err = sc.UserAMT.Publish(ctx, msg)
		}
		if err != nil {
			// залогировать
			return err
		}

		// отправить сообщение о количестве фотографий через AMT, чтобы в сервисе склада инфа обновилась
		close(sc.DirSync[dir]) // хз, но пусть будет
		delete(sc.DirSync, dir)
		delete(sc.ProcessCount, dir)
		delete(sc.ReqCount, dir)
	}
	return nil
}

func (sc *SyncController) DirSyncChannel(dir string) chan struct{} {
	sc.DirSyncMutex.RLock()
	token, ok := sc.DirSync[dir]
	sc.DirSyncMutex.RUnlock()
	if ok {
		return token
	}
	sc.DirSyncMutex.Lock()
	token, ok = sc.DirSync[dir]
	if !ok {
		token = make(chan struct{}, 1)
		sc.DirSync[dir] = token
	}
	sc.DirSyncMutex.Unlock()
	return token
}

func (sc *SyncController) ReqCountDecrement(req string) {
	sc.ReqCountMutex.Lock()
	defer sc.ReqCountMutex.Unlock()
	sc.ReqCount[req]--
}

func (sc *SyncController) ReqCountIncrement(req string) {
	sc.ReqCountMutex.Lock()
	defer sc.ReqCountMutex.Unlock()
	sc.ReqCount[req]++
}
