package template

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/snippet"
	"github.com/0xJacky/Nginx-UI/internal/template"
	"github.com/gin-gonic/gin"

	"github.com/uozi-tech/cosy"
)

func GetDefaultSiteTemplate(c *gin.Context) {
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

	content, err := ngxConfig.BuildConfig()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "ok",
		"template":  content,
		"tokenized": ngxConfig,
	})
}

func GetTemplateConfList(c *gin.Context) {
	configList, err := template.GetTemplateList("conf")

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

// GetTemplateBlockList lists the built-in block templates followed by the
// snippets of the user.
func GetTemplateBlockList(c *gin.Context) {
	configList, err := template.GetTemplateList("block")
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	configList = append(configList, snippet.TemplateInfo()...)

	c.JSON(http.StatusOK, gin.H{
		"data": configList,
	})
}

// GetTemplateBlock renders a block template, a built-in one or, with the
// origin query parameter set to custom, a snippet of the user.
func GetTemplateBlock(c *gin.Context) {
	type resp struct {
		template.ConfigInfoItem
		template.ConfigDetail
	}
	var bindData map[string]template.Variable
	_ = c.ShouldBindJSON(&bindData)
	name := c.Param("name")

	var info template.ConfigInfoItem
	if c.Query("origin") == template.OriginCustom {
		s, err := snippet.Get(name)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
		info = template.ConfigInfoItem{
			Name: s.Name, Description: s.Description, Author: s.Author,
			Filename: s.File, Variables: s.Variables, Origin: template.OriginCustom,
		}
	} else {
		info = template.GetTemplateInfo("block", name)
	}

	if bindData == nil {
		bindData = info.Variables
	}

	var (
		detail template.ConfigDetail
		err    error
	)
	if info.Origin == template.OriginCustom {
		detail, err = snippet.Render(name, bindData)
	} else {
		detail, err = template.ParseTemplate("block", name, bindData)
	}
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	info.Variables = bindData
	c.JSON(http.StatusOK, resp{
		info,
		detail,
	})
}
