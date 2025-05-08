package system

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.pfad.fr/risefront"
	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// RestoreResponse contains the response data for restore operation
type RestoreResponse struct {
	NginxUIRestored bool `json:"nginx_ui_restored"`
	NginxRestored   bool `json:"nginx_restored"`
	HashMatch       bool `json:"hash_match"`
}

// RestoreBackup restores from uploaded backup and security info
func RestoreBackup(c *gin.Context) {
	// Get restore options
	restoreNginx := c.PostForm("restore_nginx") == "true"
	restoreNginxUI := c.PostForm("restore_nginx_ui") == "true"
	verifyHash := c.PostForm("verify_hash") == "true"
	securityToken := c.PostForm("security_token") // Get concatenated key and IV
	// Get backup file
	backupFile, err := c.FormFile("backup_file")
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrBackupFileNotFound, err.Error()))
		return
	}

	// Validate security token
	if securityToken == "" {
		cosy.ErrHandler(c, backup.ErrInvalidSecurityToken)
		return
	}

	// Split security token to get Key and IV
	parts := strings.Split(securityToken, ":")
	if len(parts) != 2 {
		cosy.ErrHandler(c, backup.ErrInvalidSecurityToken)
		return
	}

	aesKey := parts[0]
	aesIv := parts[1]

	// Decode Key and IV from base64
	key, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrInvalidAESKey, err.Error()))
		return
	}

	iv, err := base64.StdEncoding.DecodeString(aesIv)
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrInvalidAESIV, err.Error()))
		return
	}

	// Create temporary directory for files
	tempDir, err := os.MkdirTemp("", "nginx-ui-restore-upload-*")
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateTempDir, err.Error()))
		return
	}
	defer os.RemoveAll(tempDir)

	// Save backup file
	backupPath := filepath.Join(tempDir, backupFile.Filename)
	if err := c.SaveUploadedFile(backupFile, backupPath); err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateBackupFile, err.Error()))
		return
	}

	// Create temporary directory for restore operation
	restoreDir, err := os.MkdirTemp("", "nginx-ui-restore-*")
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateRestoreDir, err.Error()))
		return
	}

	// Set restore options
	options := backup.RestoreOptions{
		BackupPath:     backupPath,
		AESKey:         key,
		AESIv:          iv,
		RestoreDir:     restoreDir,
		RestoreNginx:   restoreNginx,
		RestoreNginxUI: restoreNginxUI,
		VerifyHash:     verifyHash,
	}

	// Perform restore
	result, err := backup.Restore(options)
	if err != nil {
		// Clean up temporary directory on error
		os.RemoveAll(restoreDir)
		cosy.ErrHandler(c, err)
		return
	}

	// If not actually restoring anything, clean up directory to avoid disk space waste
	if !restoreNginx && !restoreNginxUI {
		defer os.RemoveAll(restoreDir)
	}

	if restoreNginx {
		go func() {
			time.Sleep(2 * time.Second)
			nginx.Restart()
		}()
	}

	if restoreNginxUI {
		go func() {
			time.Sleep(2 * time.Second)
			// gracefully restart
			risefront.Restart()
		}()
	}

	c.JSON(http.StatusOK, RestoreResponse{
		NginxUIRestored: result.NginxUIRestored,
		NginxRestored:   result.NginxRestored,
		HashMatch:       result.HashMatch,
	})
}
