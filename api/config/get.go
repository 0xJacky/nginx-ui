package config

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetConfig(c *gin.Context) {
	relativePath := helper.UnescapeURL(c.Param("path"))

	absPath := nginx.GetConfPath(relativePath)
	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "path is not under the nginx conf path",
		})
		return
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(absPath)).FirstOrInit()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, config.Config{
		Name:          stat.Name(),
		Content:       string(content),
		FilePath:      absPath,
		ModifiedAt:    stat.ModTime(),
		Dir:           filepath.Dir(relativePath),
		SyncNodeIds:   cfg.SyncNodeIds,
		SyncOverwrite: cfg.SyncOverwrite,
	})
}
