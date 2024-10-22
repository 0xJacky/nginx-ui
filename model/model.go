package model

import (
	"gorm.io/gen"
	"gorm.io/gorm"
	"time"
)

var db *gorm.DB

type Model struct {
	ID        int             `gorm:"primary_key" json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func GenerateAllModel() []any {
	return []any{
		ConfigBackup{},
		User{},
		AuthToken{},
		Cert{},
		ChatGPTLog{},
		Site{},
		Stream{},
		DnsCredential{},
		Environment{},
		Notification{},
		AcmeUser{},
		BanIP{},
		Config{},
		Passkey{},
		SiteCategory{},
	}
}

func Use(tx *gorm.DB) {
	db = tx
}

func UseDB() *gorm.DB {
	return db
}

type Pagination struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	TotalPages  int64 `json:"total_pages"`
}

type DataList struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination,omitempty"`
}

type Method interface {
	// FirstByID Where("id=@id")
	FirstByID(id int) (*gen.T, error)
	// DeleteByID update @@table set deleted_at=strftime('%Y-%m-%d %H:%M:%S','now') where id=@id
	DeleteByID(id int) error
}
