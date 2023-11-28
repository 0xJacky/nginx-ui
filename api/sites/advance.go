package sites

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"net/http"
)

func DomainEditByAdvancedMode(c *gin.Context) {
	var json struct {
		Advanced bool `json:"advanced"`
	}

	if !api.BindAndValid(c, &json) {
		return
	}

	name := c.Param("name")
	path := nginx.GetConfPath("sites-available", name)

	s := query.Site

	_, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	_, err = s.Where(s.Path.Eq(path)).Update(s.Advanced, json.Advanced)

	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})

}
