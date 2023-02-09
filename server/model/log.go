package model

import (
	"database/sql"
	"errors"
	"log"

	"github.com/gin-gonic/gin"
)

type Log struct {
	Model

	Name string `json:"name"`
	Path string `json:"path"`
}

func ListLog(c *gin.Context) (data DataList) {
	var total int64
	db.Model(&Log{}).Count(&total)
	var logs []Log

	result := db.Model(&Log{}).Scopes(orderAndPaginate(c))
	result.Find(&logs)

	data = GetListWithPagination(&logs, c, total)
	return
}

func CreateLog(l Log) error {
	lq, err := GetLog(l.Name)
	if errors.Is(err, sql.ErrNoRows) {
	}
	if lq.Name != "" {
		return errors.New("log already exists")
	}

	result := db.Create(&l)
	if result.Error != nil {
		log.Println(result.Error)
		return result.Error
	}
	return nil
}

func GetLog(name string) (log Log, err error) {
	err = db.Where("name = ?", name).First(&log).Error
	if err != nil {
		return Log{}, err
	}
	return log, nil
}

func GetLogByID(id string) (log Log, err error) {
	err = db.Where("id = ?", id).First(&log).Error
	if err != nil {
		return Log{}, err
	}
	return log, err
}

func DeleteLog(id string) error {
	return db.Where("id = ?", id).Delete(&Log{}).Error
}

func EditLog(orig Log, new Log) error {
	// update only path
	return db.Model(&orig).Updates(new).Error
}
