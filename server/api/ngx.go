package api

import (
    "github.com/0xJacky/Nginx-UI/server/pkg/nginx"
    "github.com/gin-gonic/gin"
    "net/http"
    "os"
)

func BuildNginxConfig(c *gin.Context) {
    var ngxConf nginx.NgxConfig
    if !BindAndValid(c, &ngxConf) {
        return
    }
    c.Set("maybe_error", "nginx_config_syntax_error")
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

    c.Set("maybe_error", "nginx_config_syntax_error")
    ngxConfig := nginx.ParseNgxConfigByContent(json.Content)

    c.JSON(http.StatusOK, ngxConfig)

}

func FormatNginxConfig(c *gin.Context) {
    var json struct {
        Content string `json:"content" binding:"required"`
    }

    if !BindAndValid(c, &json) {
        return
    }

    c.Set("maybe_error", "nginx_config_syntax_error")
    c.JSON(http.StatusOK, gin.H{
        "content": nginx.FmtCode(json.Content),
    })
}

func NginxStatus(c *gin.Context) {
    pidPath := nginx.GetNginxPIDPath()

    running := true
    if _, err := os.Stat(pidPath); err != nil {
        running = false
    }

    c.JSON(http.StatusOK, gin.H{
        "running": running,
    })
}

func ReloadNginx(c *gin.Context) {
    output := nginx.Reload()

    c.JSON(http.StatusOK, gin.H{
        "message": output,
        "level":   nginx.GetLogLevel(output),
    })
}

func TestNginx(c *gin.Context) {
    output := nginx.TestConf()

    c.JSON(http.StatusOK, gin.H{
        "message": output,
        "level":   nginx.GetLogLevel(output),
    })
}

func RestartNginx(c *gin.Context) {
    output := nginx.Restart()

    c.JSON(http.StatusOK, gin.H{
        "message": output,
        "level":   nginx.GetLogLevel(output),
    })
}
