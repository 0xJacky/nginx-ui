package template

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/template"
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
				Locations: []*nginx.NgxLocation{},
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
	configList, err := template.GetTemplateList("conf")

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

func GetTemplateBlockList(c *gin.Context) {
	configList, err := template.GetTemplateList("block")

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

func GetTemplateBlock(c *gin.Context) {
	type resp struct {
		template.ConfigInfoItem
		template.ConfigDetail
	}
	var bindData map[string]template.TVariable
	_ = c.ShouldBindJSON(&bindData)
	info := template.GetTemplateInfo("block", c.Param("name"))

	if bindData == nil {
		bindData = info.Variables
	}

	detail, err := template.ParseTemplate("block", c.Param("name"), bindData)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	info.Variables = bindData
	c.JSON(http.StatusOK, resp{
		info,
		detail,
	})
}
