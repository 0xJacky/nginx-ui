package config

import (
	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
)

func Rename(c *gin.Context) {
	var json struct {
		BasePath    string `json:"base_path"`
		OrigName    string `json:"orig_name"`
		NewName     string `json:"new_name"`
		SyncNodeIds []int  `json:"sync_node_ids" gorm:"serializer:json"`
	}
	if !api.BindAndValid(c, &json) {
		return
	}
	logger.Debug(json)
	if json.OrigName == json.NewName {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
		return
	}
	origFullPath := nginx.GetConfPath(json.BasePath, json.OrigName)
	newFullPath := nginx.GetConfPath(json.BasePath, json.NewName)
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
		api.ErrHandler(c, err)
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
		api.ErrHandler(c, err)
		return
	}

	// update ChatGPT records
	g := query.ChatGPTLog
	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(origFullPath)).FirstOrInit()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}
	if !stat.IsDir() {
		_, _ = g.Where(g.Name.Eq(newFullPath)).Delete()
		_, _ = g.Where(g.Name.Eq(origFullPath)).Update(g.Name, newFullPath)
		// for file, the sync policy for this file is used
		json.SyncNodeIds = cfg.SyncNodeIds
	} else {
		// is directory, update all records under the directory
		_, _ = g.Where(g.Name.Like(origFullPath+"%")).Update(g.Name, g.Name.Replace(origFullPath, newFullPath))
	}

	_, err = q.Where(q.Filepath.Eq(origFullPath)).Update(q.Filepath, newFullPath)
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	if len(json.SyncNodeIds) > 0 {
		err = config.SyncRenameOnRemoteServer(origFullPath, newFullPath, json.SyncNodeIds)
		if err != nil {
			api.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})
}
