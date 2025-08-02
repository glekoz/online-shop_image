package application

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

type SyncController struct {
	DB                DBAPI
	Storage           StorageAPI
	ImageCount        map[string]int
	ReqCountMutex     sync.RWMutex
	ReqCount          map[string]int
	ProcessCountMutex sync.RWMutex
	ProcessCount      map[string]int
	DirSyncMutex      sync.RWMutex
	DirSync           map[string]chan struct{}
}

func NewSyncController(db DBAPI, storage StorageAPI) *SyncController {
	imageCount := make(map[string]int)
	reqCount := make(map[string]int)
	processCount := make(map[string]int)
	dirSync := make(map[string]chan struct{})
	return &SyncController{DB: db, Storage: storage,
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
	var err error

	if sc.ProcessCount[dir] == sc.ReqCount[dir] {
		service := strings.ToLower(filepath.SplitList(dir)[0])
		entityID := filepath.SplitList(dir)[1]
		count := sc.ProcessCount[dir]
		// ПРИ ПОЛУЧЕНИИ ЭТОГО СООБЩЕНИЯ ОБНОВЛЯЕТСЯ СТОЛБИК С КОЛИЧЕСТВОМ ИЗОБРАЖЕНИЙ В СЕРВИСЕ
		err1 := sc.DB.SetCountAndFreeStatus(ctx, service, entityID, ImageStatusFree, count)
		err2 := sc.Storage.DeleteAll(service, entityID)
		err = errors.Join(err1, err2)

		// close(sc.DirSync[dir]) // хз, но пусть будет - закрывает канал тот, кто в него пишет
		delete(sc.DirSync, dir)
		delete(sc.ProcessCount, dir)
		delete(sc.ReqCount, dir)
	}
	return err
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
