package config

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetConfigHistory(c *gin.Context) {
	cosy.Core[model.ConfigBackup](c).
		SetEqual("filepath").
		PagingList()
}
