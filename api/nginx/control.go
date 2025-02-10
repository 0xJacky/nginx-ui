package nginx

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
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
	pidPath := nginx.GetPIDPath()
	lastOutput := nginx.GetLastOutput()

	running := true
	if fileInfo, err := os.Stat(pidPath); err != nil || fileInfo.Size() == 0 { // fileInfo.Size() == 0 no process id
		running = false
	}

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastOutput,
		"level":   nginx.GetLogLevel(lastOutput),
	})
}
