package storage

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/Gleb988/online-shop_image/internal/models"
)

type Storage struct {
	Path string // типа "/static/image" - зависит от настроек, какой том выделен в докере (в этом сервисе) под хранение изображений
}

func NewStorage(p string) (Storage, error) {
	if err := os.MkdirAll(p, 0o777); err != nil {
		return Storage{}, fmt.Errorf("%w: NewStorage: user mkdir failed: "+err.Error(), models.ErrOSAction)
	}
	return Storage{Path: p}, nil
}

// надо что-то думать насчет аргументов
// как будто нужны уже целые пути, а не составные части
func (s Storage) Save(dir, id string, img image.Image) (string, error) {
	pwd := filepath.Join(s.Path, dir)
	err := os.MkdirAll(pwd, 0o777)
	if err != nil {
		return "", fmt.Errorf("%w: Storage.Save: os.MkdirAll failed: "+err.Error(), models.ErrOSAction)
	}

	filePath := filepath.Join(pwd, id+".jpeg")
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("%w: Storage.Save: os.Create failed: "+err.Error(), models.ErrOSAction)
	}
	defer file.Close()

	if err = jpeg.Encode(file, img, &jpeg.Options{Quality: 95}); err != nil {
		file.Close() // для Windows
		os.Remove(filePath)
		return "", fmt.Errorf("%w: Storage.Save: jpeg.Encode failed: "+err.Error(), models.ErrOperationAction)
	}
	return filePath, nil
}

func (s Storage) Delete(imgPath string) error {
	filePath := filepath.Join(s.Path, imgPath)
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}
	return nil
}

// Подумать, какие аргументы принимать
func (s Storage) UpdateMainPhoto(dir, id string, img image.Image) error { // создавать новый файл во временной директории!
	newFileDirPAth := filepath.Join(s.Path, dir, "tmp")
	newFilePath := filepath.Join(newFileDirPAth, id)
	newFile, err := os.Create(newFilePath)
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}
	defer func() {
		newFile.Close()
		os.RemoveAll(newFileDirPAth)
	}()
	err = jpeg.Encode(newFile, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}
	oldFilePath := filepath.Join(s.Path, dir, id+".jpeg")
	if err = os.Rename(newFilePath, oldFilePath); err != nil {
		return fmt.Errorf("%w: "+err.Error(), models.ErrOSAction)
	}
	return nil
}

func (s Storage) ItemsInDir(dir string) (int, error) {
	pwd := filepath.Join(s.Path, dir)
	files, err := os.ReadDir(pwd)
	if err != nil {
		return 0, fmt.Errorf("%w: Storage.Save: os.ReadDir failed: "+err.Error(), models.ErrOSAction)
	}
	return len(files), nil
}

func (s Storage) GetRawImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage.GetRawImage: os.Open failed: "+err.Error(), models.ErrOSAction)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("%w: Storage.GetRawImage: image.Decode failed: "+err.Error(), models.ErrOperationAction)
	}
	return img, nil
}

/* не используется, так как будет передаваться ссылка через fileserver и httputil.ReverseProxy
func (s Storage) Get(service, id, index string) (io.ReadCloser, int64, error) {
	name := index + ".jpg"
	filePath := filepath.Join(s.Path, service, id, name)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("Storage.Save: os.Open failed: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("Storage.Save: file.Stat failed: %w", err)
	}
	size := stat.Size()
	return file, size, nil
}
*/
