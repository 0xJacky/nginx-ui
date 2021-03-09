package api

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func ErrorHandler(c *gin.Context, err error) {
	log.Println(err)
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": err.Error(),
	})
}
