package config

import (
	"net/http"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func GetConfig(c *gin.Context) {
	// An encoded value is already exact, so it must not also go through the
	// repeated-unescape loop — a filename containing a literal '%' would be
	// corrupted by it. Raw values keep the historical behaviour.
	path, encoded := helper.DecodePathParam(c.Query("path"))
	if !encoded {
		path = helper.UnescapeURL(path)
	}

	absPath, err := config.ResolveAbsoluteOrRelativeConfPath(path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	stat, err := nginx.Stat(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	content, err := nginx.ReadFile(absPath)
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
