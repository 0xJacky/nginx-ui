package config

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

func AddConfig(c *gin.Context) {
	var json struct {
		config.SyncConfigPayload
		SyncNodeIds []uint64 `json:"sync_node_ids"`
	}

	if !cosy.BindAndValid(c, &json) {
		return
	}

	name := json.Name
	content := json.Content

	// Decode paths from URL encoding
	decodedBaseDir := helper.UnescapeURL(json.BaseDir)
	decodedName := helper.UnescapeURL(name)

	dir := nginx.GetConfPath(decodedBaseDir)
	path := filepath.Join(dir, decodedName)
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		c.JSON(http.StatusForbidden, gin.H{
			"message": "filepath is not under the nginx conf path",
		})
		return
	}

	if !json.Overwrite && helper.FileExists(path) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	// check if the dir exists, if not, use mkdirAll to create the dir
	if !helper.FileExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	res := nginx.Control(nginx.Reload)
	if res.IsError() {
		res.RespError(c)
		return
	}

	q := query.Config
	_, err = q.Where(q.Filepath.Eq(path)).Delete()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	cfg := &model.Config{
		Name:          name,
		Filepath:      path,
		SyncNodeIds:   json.SyncNodeIds,
		SyncOverwrite: json.Overwrite,
	}

	err = q.Create(cfg)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = config.SyncToRemoteServer(cfg)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, config.Config{
		Name:       name,
		Content:    content,
		FilePath:   path,
		ModifiedAt: time.Now(),
	})
}
