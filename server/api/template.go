package api

import (
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/server/template"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

func GetTemplate(c *gin.Context) {
	name := c.Param("name")
	content, err := template.DistFS.ReadFile(name)

	_content := string(content)
	_content = strings.ReplaceAll(_content, "{{ HTTP01PORT }}",
		settings.ServerSettings.HTTPChallengePort)

	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"message": err.Error(),
			})
			return
		}
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "ok",
		"template": _content,
	})
}
