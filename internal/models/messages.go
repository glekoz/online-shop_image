package models

type ProcessImageMessage struct {
	ServiceDirName string `json:"service_dir_name"`
	ImagePath      string `json:"image_path"`
}

type ImageSavedMessage struct {
	DirName   string `json:"dir_name"`
	ImagePath string `json:"image_path"`
}

type CompleteImageProcessMessage struct {
	SavedItemDir string `json:"saved_item_dir"`
	TotalCount   int    `json:"total_count"`
}
