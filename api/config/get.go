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
	path := helper.UnescapeURL(c.Query("path"))

	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = nginx.GetConfPath(path)
	}

	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(config.ErrPathIsNotUnderTheNginxConfDir, absPath, nginx.GetConfPath()))
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
		Dir:           filepath.Dir(absPath),
		SyncNodeIds:   cfg.SyncNodeIds,
		SyncOverwrite: cfg.SyncOverwrite,
	})
}
