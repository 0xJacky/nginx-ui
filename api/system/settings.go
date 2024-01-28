package system

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"net/http"
	"reflect"
)

func GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"server": settings.ServerSettings,
		"nginx":  settings.NginxSettings,
		"openai": settings.OpenAISettings,
	})
}

func SaveSettings(c *gin.Context) {
	var json struct {
		Server settings.Server `json:"server"`
		Nginx  settings.Nginx  `json:"nginx"`
		Openai settings.OpenAI `json:"openai"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	fillSettings(&settings.ServerSettings, &json.Server)
	fillSettings(&settings.NginxSettings, &json.Nginx)
	fillSettings(&settings.OpenAISettings, &json.Openai)

	settings.ReflectFrom()

	err := settings.Save()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	GetSettings(c)
}

func fillSettings(targetSettings interface{}, newSettings interface{}) {
	s := reflect.TypeOf(targetSettings).Elem()
	vt := reflect.ValueOf(targetSettings).Elem()
	vn := reflect.ValueOf(newSettings).Elem()

	// copy the values from new to target settings if it is not protected
	for i := 0; i < s.NumField(); i++ {
		if s.Field(i).Tag.Get("protected") != "true" {
			vt.Field(i).Set(vn.Field(i))
		}
	}
}
