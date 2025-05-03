package nginx

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
)

// Reload reloads the nginx
func Reload(c *gin.Context) {
	output, err := nginx.Reload()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output + err.Error(),
			"level":   nginx.GetLogLevel(output),
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output + err.Error(),
			"level":   nginx.GetLogLevel(output),
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": lastOutput + err.Error(),
			"level":   nginx.GetLogLevel(lastOutput),
		})
		return
	}

	running := nginx.IsNginxRunning()

	c.JSON(http.StatusOK, gin.H{
		"running": running,
		"message": lastOutput,
		"level":   nginx.GetLogLevel(lastOutput),
	})
}
