package models

// это используется внутри сервиса изображений,
// чтобы отложить обработку
type ProcessImageMessage struct {
	Service      string `json:"service"`
	EntityID     string `json:"entity_id"`
	ImageID      string `json:"image_id"`
	IsCover      bool   `json:"is_cover"`
	TmpImagePath string `json:"image_path"`
}

// Это сообщение используется между сервисами,
// чтобы оин добавили новую запись в таблицу со списком изображений
type ImageSavedMessage struct {
	Service   string `json:"service"`
	EntityID  string `json:"entity_id"`
	ImagePath string `json:"image_path"`
}

// это финальное сообщение после обработки всех отправленных изображений
// для того, чтобы они изменили статус и количество изображений
type SaveImageMessage struct {
	Service    string `json:"service"`
	EntityID   string `json:"entity_id"`
	TotalCount int    `json:"total_count"`
}
