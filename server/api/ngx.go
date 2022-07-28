package api

import (
	"bufio"
	"github.com/0xJacky/Nginx-UI/server/tool/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
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

	scanner := bufio.NewScanner(strings.NewReader(json.Content))

	ngxConfig, err := nginx.ParseNgxConfigByScanner("", scanner)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, ngxConfig)

}
