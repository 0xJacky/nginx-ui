package streams

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func Duplicate(c *gin.Context) {
	// Source name
	name := helper.UnescapeURL(c.Param("name"))

	// Destination name
	var json struct {
		Name string `json:"name" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := stream.Duplicate(name, json.Name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	dst, err := stream.ResolveAvailablePath(json.Name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dst": dst,
	})
}
