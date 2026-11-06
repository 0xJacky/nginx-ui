package config

import (
	"net/http"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"gorm.io/gen/field"
)

type EditConfigJson struct {
	Content string `json:"content" binding:"required"`
}

func EditConfig(c *gin.Context) {
	var json struct {
		Content       string   `json:"content"`
		Path          string   `json:"path"`
		SyncOverwrite bool     `json:"sync_overwrite"`
		SyncNodeIds   []uint64 `json:"sync_node_ids"`
		// AllowRestrictedDirectives asks to keep the restricted js_*, perl*
		// and load_module statements in the content. It is honoured only for
		// an interactive administrator with a verified two-factor session.
		AllowRestrictedDirectives bool `json:"allow_restricted_directives"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	absPath, err := config.ResolveAbsoluteOrRelativeConfPath(json.Path)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	exists, err := nginx.Exists(absPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	if rejectManagedSiteFile(c, absPath) {
		return
	}

	content := json.Content
	allowRestricted := api.AllowRestrictedDirectives(c, json.AllowRestrictedDirectives)

	// Validated before the database work below, so a rejected content leaves
	// neither the file nor the configuration record touched. config.Save
	// validates again with the same decision, which keeps the guarantee for
	// its other callers.
	if err = config.ValidateConfigFileWithRestricted(absPath, content, allowRestricted); err != nil {
		api.ConfigSaveError(c, absPath, content, err)
		return
	}

	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Filepath: absPath,
		Name:     filepath.Base(absPath),
	})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
	if err != nil {
		return
	}

	// Update database record
	_, err = q.Where(q.Filepath.Eq(absPath)).
		Select(q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			SyncNodeIds:   json.SyncNodeIds,
			SyncOverwrite: json.SyncOverwrite,
		})
	if err != nil {
		return
	}

	cfg.SyncNodeIds = json.SyncNodeIds
	cfg.SyncOverwrite = json.SyncOverwrite

	err = config.SaveWithOptions(c.Request.Context(), absPath, content, cfg, config.SaveOptions{
		UserName:                  api.CurrentUser(c).Name,
		AllowRestrictedDirectives: allowRestricted,
	})
	if err != nil {
		api.ConfigSaveError(c, absPath, content, err)
		return
	}

	api.RecordRestrictedDirectiveSave(c, absPath, content, allowRestricted)

	c.JSON(http.StatusOK, config.Config{
		Name:          filepath.Base(absPath),
		Content:       content,
		FilePath:      absPath,
		ModifiedAt:    time.Now(),
		Dir:           filepath.Dir(absPath),
		SyncNodeIds:   cfg.SyncNodeIds,
		SyncOverwrite: cfg.SyncOverwrite,
	})
}
