package storage

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
)

type Storage struct {
	Path string // типа "/static/image" - зависит от настроек, какой том выделен в докере (в этом сервисе) под хранение изображений
}

func NewStorage(p string) (Storage, error) {
	if err := os.MkdirAll(p, 0o777); err != nil {
		return Storage{}, fmt.Errorf("NewStorage: os.MkdirAll: %w", err)
	}
	return Storage{Path: p}, nil
}

// надо что-то думать насчет аргументов
// как будто нужны уже целые пути, а не составные части
func (s Storage) Save(ctx context.Context, service, entityID, imageID string, img image.Image) (string, error) {
	type Result struct {
		imagePath string
		err       error
	}
	resultChan := make(chan Result, 1)

	go func(ch chan<- Result) {
		//result := Result{}
		defer close(resultChan)

		pwd := filepath.Join(s.Path, service, entityID)
		err := os.MkdirAll(pwd, 0o755)
		if err != nil {
			ch <- Result{"", fmt.Errorf("Storage.Save: os.MkdirAll: %w", err)}
			return
		}

		imagePath := filepath.Join(pwd, imageID+".jpeg")
		file, err := os.Create(imagePath)
		if err != nil {
			ch <- Result{"", fmt.Errorf("Storage.Save: os.Create: %w", err)}
			return
		}
		defer func() {
			file.Close()
			if ctx.Err() != nil || err != nil {
				os.Remove(imagePath) // удаляем файл, если произошла ошибка
			}
		}()

		if err = ctx.Err(); err != nil {
			ch <- Result{"", fmt.Errorf("Storage.Save: context: %w", err)}
			return
		}

		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
		if err != nil {
			ch <- Result{"", fmt.Errorf("Storage.Save: jpeg.Encode: %w", err)}
			return
		}
		ch <- Result{imagePath, nil} // возвращаем путь к файлу, чтобы можно было использовать в других методах
	}(resultChan)
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("Storage.Save context: %w", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return "", fmt.Errorf("Storage.Save: %w", result.err)
		}
		return result.imagePath, nil
	}
}

/*
defer func() {
			//resultChan <- result
			close(resultChan)
			if ctx.Err() != nil || err != nil {
				// логировать ошибку ниже
				os.Remove(result.filePath) // удаляем файл, если произошла ошибка
			}
		}()
*/

func (s Storage) Delete(path string) error {
	if path == "" {
		return fmt.Errorf("invalid input")
	}
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("Storage.Delete: os.Remove: %w", err)
	}
	return nil
}

func (s Storage) DeleteAll(service, entityID string) error {
	path := filepath.Join(s.Path, service, entityID)
	return os.RemoveAll(path)
}

func (s Storage) GetRawImage(imagePath string) (image.Image, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("Storage.GetRawImage: os.Open: %w", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("Storage.GetRawImage: image.Decode: %w", err)
	}
	return img, nil
}

/*

func (s Storage) ItemsInDir(dir string) (int, error) {
	pwd := filepath.Join(s.Path, dir)
	files, err := os.ReadDir(pwd)
	if err != nil {
		return 0, fmt.Errorf("Storage.ItemsInDir: os.ReadDir: %w", err)
	}
	return len(files), nil
}

не используется, так как будет передаваться ссылка через fileserver и httputil.ReverseProxy
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

// не используется, так как перенес в БД сервиса переставление флага с одного изображения на другое
// Подумать, какие аргументы принимать
func (s Storage) UpdateMainPhoto(ctx context.Context, dir, id string, img image.Image) error { // создавать новый файл во временной директории!
	newFileDirPath := filepath.Join(s.Path, dir, "tmp")
	newFilePath := filepath.Join(newFileDirPath, id)
	newFile, err := os.Create(newFilePath)
	if err != nil {
		return fmt.Errorf("Storage.UpdateMainPhoto: os.Create: %w", err)
	}
	defer func() {
		newFile.Close()
		os.RemoveAll(newFileDirPath)
	}()

	if err = ctx.Err(); err != nil {
		return fmt.Errorf("Storage.UpdateMainPhoto: context: %w", ctx.Err())
	}

	err = jpeg.Encode(newFile, img, &jpeg.Options{Quality: 90})
	if err != nil {
		return fmt.Errorf("Storage.UpdateMainPhoto: jpeg.Encode: %w", err)
	}
	oldFilePath := filepath.Join(s.Path, dir, id)
	if err = os.Rename(newFilePath, oldFilePath); err != nil {
		return fmt.Errorf("Storage.UpdateMainPhoto: os.Rename: %w", err)
	}
	return nil
}
*/
