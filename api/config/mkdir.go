package config

import (
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
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
	decodedBasePath := helper.UnescapeURL(json.BasePath)

	decodedFolderName := helper.UnescapeURL(json.FolderName)

	fullPath, err := config.ResolveConfPath(decodedBasePath, decodedFolderName)
	if err != nil {
		cosy.ErrHandler(c, err)
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
