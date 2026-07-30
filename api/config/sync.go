package config

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/clustersync"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// SyncConfigBatch is the receiving end of a directory deployment. It writes
// every file of the payload and reloads Nginx once, instead of reloading per
// file like the single file endpoint does.
func SyncConfigBatch(c *gin.Context) {
	var json struct {
		Files     []clustersync.ConfigFile `json:"files"`
		Overwrite bool                     `json:"overwrite"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	written := 0
	skipped := 0
	for _, file := range json.Files {
		path, err := config.ResolveConfPath(helper.UnescapeURL(file.BaseDir), helper.UnescapeURL(file.Name))
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		if !json.Overwrite && helper.FileExists(path) {
			skipped++
			continue
		}

		if err = config.ValidateConfigFile(path, file.Content); err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		if err = os.WriteFile(path, []byte(file.Content), 0644); err != nil {
			cosy.ErrHandler(c, err)
			return
		}

		written++
	}

	if written > 0 {
		if res := nginx.Control(nginx.Reload); res.IsError() {
			res.RespError(c)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"written": written,
		"skipped": skipped,
	})
}

// SyncConfigDirectory replicates a whole configuration directory to the
// selected nodes rather than a single file.
func SyncConfigDirectory(c *gin.Context) {
	var json struct {
		Dir           string   `json:"dir"`
		SyncNodeIds   []uint64 `json:"sync_node_ids" binding:"required"`
		SyncOverwrite bool     `json:"sync_overwrite"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	dir, err := config.ResolveConfPath(helper.UnescapeURL(json.Dir))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	summary, err := clustersync.SyncDirectory(c, dir, json.SyncNodeIds, json.SyncOverwrite)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Remember the deployment targets on the directory so files created below it
	// inherit them and keep replicating without further configuration.
	if err = config.SaveDirectorySyncTargets(dir, json.SyncNodeIds, json.SyncOverwrite); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}
