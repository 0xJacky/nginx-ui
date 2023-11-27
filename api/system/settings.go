package system

import (
    "github.com/0xJacky/Nginx-UI/api"
    "github.com/0xJacky/Nginx-UI/settings"
    "github.com/gin-gonic/gin"
    "net/http"
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

    settings.ServerSettings = json.Server
    settings.NginxSettings = json.Nginx
    settings.OpenAISettings = json.Openai

    settings.ReflectFrom()

    err := settings.Save()
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    GetSettings(c)
}
