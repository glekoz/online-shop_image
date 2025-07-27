package application

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	amt "github.com/Gleb988/online-shop_amt"
	"github.com/Gleb988/online-shop_image/internal/models"
)

type SyncController struct {
	ProductAMT        amt.AMT
	UserAMT           amt.AMT
	ImageCount        map[string]int
	ReqCountMutex     sync.RWMutex
	ReqCount          map[string]int
	ProcessCountMutex sync.RWMutex
	ProcessCount      map[string]int
	DirSyncMutex      sync.Mutex
	DirSync           map[string]chan struct{}
}

func NewSyncController(s StorageAPI, product, user amt.AMT) *SyncController {
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
	sc.ProcessCount[dir]++
	sc.ProcessCountMutex.Unlock()
}

func (sc *SyncController) SyncMemoryClean(dir string) error {
	sc.ProcessCountMutex.RLock()
	sc.ReqCountMutex.RLock()

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
			return fmt.Errorf("%w: "+err.Error(), models.ErrOperationAction)
		}
		if serviceName == "product" {
			err := sc.ProductAMT.Publish(msg)
			if err != nil {
				// залогировать
				return fmt.Errorf("%w: "+err.Error(), models.ErrNetworkAction)
			}
		} else {
			err := sc.UserAMT.Publish(msg)
			if err != nil {
				// залогировать
				return fmt.Errorf("%w: "+err.Error(), models.ErrNetworkAction)
			}
		}
		// отправить сообщение о количестве фотографий через AMT, чтобы в сервисе склада инфа обновилась
		close(sc.DirSync[dir]) // хз, но пусть будет
		delete(sc.DirSync, dir)
		delete(sc.ProcessCount, dir)
		delete(sc.ReqCount, dir)
	}
	sc.ReqCountMutex.RUnlock()
	sc.ProcessCountMutex.RUnlock()
	return nil
}

func (sc *SyncController) DirSyncChannel(dir string) chan struct{} {
	sc.DirSyncMutex.Lock()
	token, ok := sc.DirSync[dir]
	if !ok {
		token = make(chan struct{}, 1)
		sc.DirSync[dir] = token
	}
	sc.DirSyncMutex.Unlock()
	return token
}

func (sc *SyncController) ReqCountDecrement(req string) {
	sc.ReqCountMutex.Lock()
	sc.ReqCount[req]--
	sc.ReqCountMutex.Unlock()
}

func (sc *SyncController) ReqCountIncrement(req string) {
	sc.ReqCountMutex.Lock()
	sc.ReqCount[req]++
	sc.ReqCountMutex.Unlock()
}
