package config

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func Rename(c *gin.Context) {
	var json struct {
		BasePath    string   `json:"base_path"`
		OrigName    string   `json:"orig_name"`
		NewName     string   `json:"new_name"`
		SyncNodeIds []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	if json.OrigName == json.NewName {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
		return
	}

	// Decode paths from URL encoding
	decodedBasePath := helper.UnescapeURL(json.BasePath)

	decodedOrigName := helper.UnescapeURL(json.OrigName)

	decodedNewName, err := url.QueryUnescape(json.NewName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	origFullPath := nginx.GetConfPath(decodedBasePath, decodedOrigName)
	newFullPath := nginx.GetConfPath(decodedBasePath, decodedNewName)
	if !helper.IsUnderDirectory(origFullPath, nginx.GetConfPath()) ||
		!helper.IsUnderDirectory(newFullPath, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "you are not allowed to rename a file " +
				"outside of the nginx config path",
		})
		return
	}

	stat, err := os.Stat(origFullPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if helper.FileExists(newFullPath) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "target file already exists",
		})
		return
	}

	err = os.Rename(origFullPath, newFullPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// update LLM records
	g := query.LLMSession
	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(origFullPath)).FirstOrInit()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if !stat.IsDir() {
		_, _ = g.Where(g.Path.Eq(newFullPath)).Delete()
		_, _ = g.Where(g.Path.Eq(origFullPath)).Update(g.Path, newFullPath)
		// for file, the sync policy for this file is used
		json.SyncNodeIds = cfg.SyncNodeIds
	} else {
		// is directory, update all records under the directory
		_, _ = g.Where(g.Path.Like(origFullPath+"%")).Update(g.Path, g.Path.Replace(origFullPath, newFullPath))
	}

	_, err = q.Where(q.Filepath.Eq(origFullPath)).Updates(&model.Config{
		Filepath: newFullPath,
		Name:     json.NewName,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	b := query.ConfigBackup
	_, _ = b.Where(b.FilePath.Eq(origFullPath)).Updates(map[string]interface{}{
		"filepath": newFullPath,
		"name":     json.NewName,
	})

	if len(json.SyncNodeIds) > 0 {
		err = config.SyncRenameOnRemoteServer(origFullPath, newFullPath, json.SyncNodeIds)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"path": strings.TrimLeft(filepath.Join(json.BasePath, json.NewName), "/"),
	})
}
