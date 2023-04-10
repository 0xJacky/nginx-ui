package api

import (
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/0xJacky/Nginx-UI/server/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

func GetTemplate(c *gin.Context) {
	var ngxConfig *nginx.NgxConfig

	ngxConfig = &nginx.NgxConfig{
		Servers: []*nginx.NgxServer{
			{
				Directives: []*nginx.NgxDirective{
					{
						Directive: "listen",
						Params:    "80",
					},
					{
						Directive: "listen",
						Params:    "[::]:80",
					},
					{
						Directive: "server_name",
					},
					{
						Directive: "root",
					},
					{
						Directive: "index",
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ok",
		"template":  ngxConfig.BuildConfig(),
		"tokenized": ngxConfig,
	})
}

func GetTemplateConfList(c *gin.Context) {
	configList, err := service.GetTemplateList("conf")

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

func GetTemplateBlockList(c *gin.Context) {
	configList, err := service.GetTemplateList("block")

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

func GetTemplateBlock(c *gin.Context) {
	type resp struct {
		service.ConfigInfoItem
		service.ConfigDetail
	}
	var bindData map[string]service.TVariable
	_ = c.ShouldBindJSON(&bindData)
	info := service.GetTemplateInfo("block", c.Param("name"))

	if bindData == nil {
		bindData = info.Variables
	}

	detail, err := service.ParseTemplate("block", c.Param("name"), bindData)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	info.Variables = bindData
	c.JSON(http.StatusOK, resp{
		info,
		detail,
	})
}
