package models

type EntityState struct {
	Service    string
	EntityID   string
	ImageCount int
	Status     string
	MaxCount   int
}

type EntityImage struct {
	Service   string
	EntityID  string
	ImagePath string
	IsCover   bool
}
