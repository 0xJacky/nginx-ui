package streams

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func AdvancedEdit(c *gin.Context) {
	var json struct {
		Advanced bool `json:"advanced"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	name := helper.UnescapeURL(c.Param("name"))
	path, err := stream.ResolveAvailablePath(name)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	s := query.Stream

	_, err = s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	_, err = s.Where(s.Path.Eq(path)).Update(s.Advanced, json.Advanced)

	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
