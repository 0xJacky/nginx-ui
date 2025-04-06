package model

type ConfigBackup struct {
	Model
	Name     string `json:"name"`
	FilePath string `json:"filepath" gorm:"column:filepath"`
	Content  string `json:"content" gorm:"type:text"`
}
