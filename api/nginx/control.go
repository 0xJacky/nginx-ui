package nginx

import (
	nginx2 "github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func Reload(c *gin.Context) {
	output := nginx2.Reload()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}

func Test(c *gin.Context) {
	output := nginx2.TestConf()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}

func Restart(c *gin.Context) {
	output := nginx2.Restart()
	c.JSON(http.StatusOK, gin.H{
		"message": output,
		"level":   nginx2.GetLogLevel(output),
	})
}
