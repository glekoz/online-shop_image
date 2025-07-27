package storage

type S3Storage struct {
	Path string // типа "/static/image" - зависит от настроек, какой том выделен в докере (в этом сервисе) под хранение изображений
}
