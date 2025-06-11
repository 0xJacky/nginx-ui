package config

import (
	"net/http"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// DeleteConfig handles the deletion of configuration files or directories
func DeleteConfig(c *gin.Context) {
	var json struct {
		BasePath    string   `json:"base_path"`
		Name        string   `json:"name" binding:"required"`
		SyncNodeIds []uint64 `json:"sync_node_ids" gorm:"serializer:json"`
	}
	if !cosy.BindAndValid(c, &json) {
		return
	}

	// Decode paths from URL encoding
	decodedBasePath := helper.UnescapeURL(json.BasePath)
	decodedName := helper.UnescapeURL(json.Name)

	fullPath := nginx.GetConfPath(decodedBasePath, decodedName)

	// Check if path is under nginx config directory
	if err := config.ValidateDeletePath(fullPath); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Check if trying to delete protected paths
	if config.IsProtectedPath(fullPath, decodedName) {
		cosy.ErrHandler(c, config.ErrCannotDeleteProtectedPath)
		return
	}

	// Check if file/directory exists
	stat, err := config.CheckFileExists(fullPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Delete the file or directory
	err = os.RemoveAll(fullPath)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Clean up database records
	if err := config.CleanupDatabaseRecords(fullPath, stat.IsDir()); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Sync deletion to remote servers if configured
	if len(json.SyncNodeIds) > 0 {
		err = config.SyncDeleteOnRemoteServer(fullPath, json.SyncNodeIds)
		if err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deleted successfully",
	})
}
