package nginx

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func BuildNginxConfig(c *gin.Context) {
	var ngxConf nginx.NgxConfig
	if !api.BindAndValid(c, &ngxConf) {
		return
	}
	content, err := ngxConf.BuildConfig()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": content,
	})
}

func TokenizeNginxConfig(c *gin.Context) {
	var json struct {
		Content string `json:"content" binding:"required"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	ngxConfig, err := nginx.ParseNgxConfigByContent(json.Content)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, ngxConfig)

}

func FormatNginxConfig(c *gin.Context) {
	var json struct {
		Content string `json:"content" binding:"required"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}
	content, err := nginx.FmtCode(json.Content)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": content,
	})
}
