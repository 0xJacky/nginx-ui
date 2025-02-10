package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"net/http"
)

func DuplicateSite(c *gin.Context) {
	// Source name
	src := c.Param("name")

	// Destination name
	var json struct {
		Name string `json:"name" binding:"required"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	err := site.Duplicate(src, json.Name)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
