package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
)

func DuplicateSite(c *gin.Context) {
	// Source name
	name := c.Param("name")

	// Destination name
	var json struct {
		Name string `json:"name" binding:"required"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	src := nginx.GetConfPath("sites-available", name)
	dst := nginx.GetConfPath("sites-available", json.Name)

	if helper.FileExists(dst) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	_, err := helper.CopyFile(src, dst)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dst": dst,
	})
}
