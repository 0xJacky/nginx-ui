package config

import (
	"net/http"
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

	dir, err := config.ResolveConfPath(decodedBaseDir)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	path, err := config.ResolveConfPath(decodedBaseDir, decodedName)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	err = config.ValidateConfigFile(path, content)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if !json.Overwrite {
		exists, existsErr := nginx.Exists(path)
		if existsErr != nil {
			cosy.ErrHandler(c, existsErr)
			return
		}
		if exists {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "File exists",
			})
			return
		}
	}

	// check if the dir exists, if not, use mkdirAll to create the dir
	dirExists, err := nginx.Exists(dir)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if !dirExists {
		err = nginx.MkdirAll(dir, 0755)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent mutation cannot make this request fail on somebody else's file.
	release := config.LockApply()
	defer release()

	tx := &config.FileTransaction{}
	if err = tx.Write(path, []byte(content), 0644); err != nil {
		cosy.ErrHandler(c, config.RollbackError(err, tx.Rollback))
		return
	}

	// A file Nginx rejects must not survive on disk. The running instance keeps
	// its valid in-memory configuration, so an untested write only breaks the
	// next Nginx start. A newly created file is removed by the rollback.
	if err = tx.TestAndReload(); err != nil {
		cosy.ErrHandler(c, err)
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
