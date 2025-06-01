package streams

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
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

	src := nginx.GetConfPath("streams-available", name)
	dst := nginx.GetConfPath("streams-available", json.Name)

	if helper.FileExists(dst) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	_, err := helper.CopyFile(src, dst)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dst": dst,
	})
}
