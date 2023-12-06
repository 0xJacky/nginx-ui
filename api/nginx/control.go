package nginx

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
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
	output := nginx.Restart()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx.GetLogLevel(output),
	})
}
