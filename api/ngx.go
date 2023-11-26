package api

import (
    nginx2 "github.com/0xJacky/Nginx-UI/internal/nginx"
    "github.com/gin-gonic/gin"
    "net/http"
    "os"
)

func BuildNginxConfig(c *gin.Context) {
	var ngxConf nginx2.NgxConfig
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
	ngxConfig := nginx2.ParseNgxConfigByContent(json.Content)

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
		"content": nginx2.FmtCode(json.Content),
	})
}

func NginxStatus(c *gin.Context) {
	pidPath := nginx2.GetNginxPIDPath()

	running := true
	if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 { // fileInfo.Size() == 0 no process id
		running = false
	}

	c.JSON(http.StatusOK, gin.H{
		"running": running,
	})
}

func ReloadNginx(c *gin.Context) {
	output := nginx2.Reload()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}

func TestNginx(c *gin.Context) {
	output := nginx2.TestConf()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}

func RestartNginx(c *gin.Context) {
	output := nginx2.Restart()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}
