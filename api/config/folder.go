package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
)

func Mkdir(c *gin.Context) {
	var json struct {
		BasePath   string `json:"base_path"`
		FolderName string `json:"folder_name"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}
	fullPath := nginx.GetConfPath(json.BasePath, json.FolderName)
	if !helper.IsUnderDirectory(fullPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "You are not allowed to create a folder " +
				"outside of the nginx configuration directory",
		})
		return
	}
	err := os.Mkdir(fullPath, 0755)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
