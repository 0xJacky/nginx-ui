package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

func Reload(c *gin.Context) {
	output := nginx.Reload()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx.GetLogLevel(output),
	})
}

func Test(c *gin.Context) {
	output := nginx.TestConf()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx.GetLogLevel(output),
	})
}

func Restart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
	go nginx.Restart()
}

func Status(c *gin.Context) {
	lastOutput := nginx.GetLastOutput()

	running := nginx.IsNginxRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastOutput,
		"level":   nginx.GetLogLevel(lastOutput),
	})
}
