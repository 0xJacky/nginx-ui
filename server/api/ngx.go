package api

import (
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func BuildNginxConfig(c *gin.Context) {
	var ngxConf nginx.NgxConfig
	if !BindAndValid(c, &ngxConf) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": ngxConf.BuildConfig(),
	})
}

func TokenizeNginxConfig(c *gin.Context) {
	var json struct {
		Content string `json:"content" binding:"required"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	ngxConfig := nginx.ParseNgxConfigByContent(json.Content)

	c.JSON(http.StatusOK, ngxConfig)

}
