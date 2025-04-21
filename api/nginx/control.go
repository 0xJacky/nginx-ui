package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// Reload reloads the nginx
func Reload(c *gin.Context) {
	output, err := nginx.Reload()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx.GetLogLevel(output),
	})
}

// TestConfig tests the nginx config
func TestConfig(c *gin.Context) {
	output, err := nginx.TestConfig()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx.GetLogLevel(output),
	})
}

// Restart restarts the nginx
func Restart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
	go nginx.Restart()
}

// Status returns the status of the nginx
func Status(c *gin.Context) {
	lastOutput, err := nginx.GetLastOutput()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	running := nginx.IsNginxRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastOutput,
		"level":   nginx.GetLogLevel(lastOutput),
	})
}
