package query

import (
	"gorm.io/gorm"
)

func Init(db *gorm.DB) {
	SetDefault(db)
}
