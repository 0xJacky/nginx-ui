package system

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func GetProcessStats(c *gin.Context) {
	pid := os.Getpid()

	c.JSON(http.StatusOK, gin.H{
		"pid": pid,
	})
}
