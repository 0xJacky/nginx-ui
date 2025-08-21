package system

import (
	"net/http"

	"code.pfad.fr/risefront"
	"github.com/gin-gonic/gin"
)

func Restart(c *gin.Context) {
	risefront.Restart()

	c.JSON(http.StatusOK, gin.H{
		"message": "restarting...",
	})
}
