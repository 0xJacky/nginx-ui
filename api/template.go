package api

import (
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GetTemplate(c *gin.Context) {
	name := c.Param("name")
	path := filepath.Join("template", name)
	content, err := ioutil.ReadFile(path)

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
