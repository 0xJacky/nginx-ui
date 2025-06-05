package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

// Reload reloads the nginx
func Reload(c *gin.Context) {
	nginx.Control(nginx.Reload).Resp(c)
}

// TestConfig tests the nginx config
func TestConfig(c *gin.Context) {
	lastResult := nginx.Control(nginx.TestConfig)
	c.JSON(http.StatusOK, gin.H{
		"message": lastResult.GetOutput(),
		"level":   lastResult.GetLevel(),
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
	lastResult := nginx.GetLastResult()

	running := nginx.IsRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastResult.GetOutput(),
		"level":   lastResult.GetLevel(),
	})
}
