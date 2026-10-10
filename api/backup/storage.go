package backup

import (
	"context"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

const (
	// pluginStorageValidateTimeout bounds the configuration check on save.
	pluginStorageValidateTimeout = 30 * time.Second
	// pluginStorageListTimeout bounds a listing, which reaches the backend.
	pluginStorageListTimeout = time.Minute
	// pluginStorageRestoreTimeout covers fetching an archive and its key.
	pluginStorageRestoreTimeout = 25 * time.Minute
)

// GetStorageBackends lists the built-in storage and the backends plugins
// provide, with the schema of their configuration form.
func GetStorageBackends(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": backup.StorageBackends()})
}

// validatePluginStorage lets the plugin behind a storage backend check the
// configuration when a task is created or its storage changes. The payload
// may carry only some fields on modify, so the stored ones fill the gaps.
func validatePluginStorage(ctx *cosy.Ctx[model.AutoBackup]) {
	_, hasType := ctx.Payload["storage_type"]
	_, hasPath := ctx.Payload["storage_path"]
	_, hasConfig := ctx.Payload["storage_config"]
	if !hasType && !hasPath && !hasConfig {
		return
	}

	merged := ctx.Model
	if !hasType {
		merged.StorageType = ctx.OriginModel.StorageType
	}
	if !hasPath {
		merged.StoragePath = ctx.OriginModel.StoragePath
	}
	if !hasConfig {
		merged.StorageConfig = ctx.OriginModel.StorageConfig
	}
	if !merged.IsPluginStorage() {
		return
	}

	validateCtx, cancel := context.WithTimeout(ctx.RequestContext(), pluginStorageValidateTimeout)
	defer cancel()
	if err := backup.ValidatePluginStorage(validateCtx, merged.StorageType, merged.StoragePath, merged.StorageConfig); err != nil {
		ctx.AbortWithError(err)
	}
}

// TestPluginStorage checks a plugin storage backend configuration before it
// is saved: the plugin validates it and lists the key prefix of the task.
func TestPluginStorage(c *gin.Context) {
	var autoBackup model.AutoBackup
	if !cosy.BindAndValid(c, &autoBackup) {
		return
	}
	if !autoBackup.IsPluginStorage() {
		cosy.ErrHandler(c, backup.ErrPluginStorageRequired)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), pluginStorageListTimeout)
	defer cancel()
	stored, err := backup.TestPluginStorage(ctx, &autoBackup)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Storage backend test successful", "stored": stored})
}

// ListStoredBackups lists the runs the plugin storage backend of a task
// keeps, newest first.
func ListStoredBackups(c *gin.Context) {
	autoBackup, err := backup.GetAutoBackupByID(cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), pluginStorageListTimeout)
	defer cancel()
	runs, err := backup.ListStoredBackups(ctx, autoBackup)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": runs})
}

// DeleteStoredBackup removes one run of a task from its plugin storage
// backend, the archive and its key file. The run is named by the key of its
// archive in the "key" query parameter.
func DeleteStoredBackup(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrStoredBackupNotFound, key))
		return
	}
	autoBackup, err := backup.GetAutoBackupByID(cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), pluginStorageListTimeout)
	defer cancel()
	if err = backup.DeleteStoredBackup(ctx, autoBackup, key); err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// restoreStoredRequest selects the run and what it replaces.
type restoreStoredRequest struct {
	Key            string `json:"key" binding:"required"`
	RestoreNginx   bool   `json:"restore_nginx"`
	RestoreNginxUI bool   `json:"restore_nginx_ui"`
	VerifyHash     bool   `json:"verify_hash"`
}

// RestoreStoredBackup fetches one run of a task from its plugin storage
// backend and restores it like an uploaded backup.
func RestoreStoredBackup(c *gin.Context) {
	var req restoreStoredRequest
	if !cosy.BindAndValid(c, &req) {
		return
	}
	autoBackup, err := backup.GetAutoBackupByID(cast.ToUint64(c.Param("id")))
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), pluginStorageRestoreTimeout)
	defer cancel()
	result, err := backup.RestoreStoredBackup(ctx, autoBackup, req.Key, backup.RestoreStoredOptions{
		RestoreNginx:   req.RestoreNginx,
		RestoreNginxUI: req.RestoreNginxUI,
		VerifyHash:     req.VerifyHash,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	finishRestore(c, req.RestoreNginx, req.RestoreNginxUI, result)
}
