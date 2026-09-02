package config

import (
	"net/http"
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

	// One rejected file must not discard the rest of the batch: a real
	// configuration directory always holds something the validator dislikes, and
	// dropping the whole sync over it would make the feature useless.
	written := 0
	skipped := 0
	failures := make([]gin.H, 0)

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent mutation cannot make this batch fail on somebody else's file.
	release := config.LockApply()
	defer release()

	// Every written file is snapshotted so a configuration Nginx rejects can be
	// undone completely instead of waiting on disk for the next Nginx start.
	tx := &config.FileTransaction{}

	for _, file := range json.Files {
		relativePath := filepath.ToSlash(filepath.Join(file.BaseDir, file.Name))

		path, err := config.ResolveConfPath(helper.UnescapeURL(file.BaseDir), helper.UnescapeURL(file.Name))
		if err != nil {
			failures = append(failures, gin.H{"path": relativePath, "error": err.Error()})
			continue
		}

		if !json.Overwrite {
			exists, existsErr := nginx.Exists(path)
			if existsErr != nil {
				failures = append(failures, gin.H{"path": relativePath, "error": existsErr.Error()})
				continue
			}
			if exists {
				skipped++
				continue
			}
		}

		if err = config.ValidateConfigFile(path, file.Content); err != nil {
			failures = append(failures, gin.H{"path": relativePath, "error": err.Error()})
			continue
		}

		if err = nginx.MkdirAll(filepath.Dir(path), 0755); err != nil {
			failures = append(failures, gin.H{"path": relativePath, "error": err.Error()})
			continue
		}

		if err = tx.Write(path, []byte(file.Content), 0644); err != nil {
			failures = append(failures, gin.H{"path": relativePath, "error": err.Error()})
			continue
		}

		written++
	}

	switch {
	case written > 0:
		// `nginx -t` cannot attribute a failure to a single file, so a batch the
		// receiver cannot load is rolled back as a whole and reported back to the
		// controller node instead of being left behind half applied.
		if err := tx.TestAndReload(); err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	case tx.Len() > 0:
		// Nothing was applied, but a write that failed midway can still have
		// truncated a file. Undo whatever reached the disk.
		if err := tx.Rollback(); err != nil {
			failures = append(failures, gin.H{"path": "rollback", "error": err.Error()})
		}
	}

	// Only a batch where nothing could be applied is an error.
	status := http.StatusOK
	if written == 0 && skipped == 0 && len(failures) > 0 {
		status = http.StatusInternalServerError
	}

	c.JSON(status, gin.H{
		"message":  "ok",
		"written":  written,
		"skipped":  skipped,
		"failures": failures,
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
