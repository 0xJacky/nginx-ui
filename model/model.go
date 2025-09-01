package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gen"
	"gorm.io/gorm"
)

var db *gorm.DB

type Model struct {
	ID        uint64          `gorm:"primary_key" json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// BaseModelUUID defines a base model with UUID as the primary key.
type BaseModelUUID struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate will set a UUID rather than numeric ID.
func (base *BaseModelUUID) BeforeCreate(tx *gorm.DB) (err error) {
	if base.ID == uuid.Nil {
		base.ID = uuid.New()
	}
	return
}

func GenerateAllModel() []any {
	return []any{
		ConfigBackup{},
		User{},
		AuthToken{},
		Cert{},
		LLMMessages{},
		Site{},
		Stream{},
		DnsCredential{},
		Node{},
		Notification{},
		AcmeUser{},
		BanIP{},
		Config{},
		Passkey{},
		Namespace{},
		ExternalNotify{},
		AutoBackup{},
		SiteConfig{},
		NginxLogIndex{},
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
	FirstByID(id uint64) (*gen.T, error)
	// DeleteByID update @@table set deleted_at=strftime('%Y-%m-%d %H:%M:%S','now') where id=@id
	DeleteByID(id uint64) error
}
