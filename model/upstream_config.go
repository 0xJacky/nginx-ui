package model

type UpstreamConfig struct {
	Model
	Socket  string `json:"socket" gorm:"uniqueIndex"` // host:port address
	Enabled bool   `json:"enabled" gorm:"default:true"`
}
