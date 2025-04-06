package config

import (
	"net/http"
	"net/url"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func Mkdir(c *gin.Context) {
	var json struct {
		BasePath   string `json:"base_path"`
		FolderName string `json:"folder_name"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Ensure paths are properly URL unescaped
	decodedBasePath, err := url.QueryUnescape(json.BasePath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	decodedFolderName, err := url.QueryUnescape(json.FolderName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	fullPath := nginx.GetConfPath(decodedBasePath, decodedFolderName)
	if !helper.IsUnderDirectory(fullPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "You are not allowed to create a folder " +
				"outside of the nginx configuration directory",
		})
		return
	}
	err = os.Mkdir(fullPath, 0755)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
