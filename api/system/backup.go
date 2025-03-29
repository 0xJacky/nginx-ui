package system

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/api"
	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/gin-gonic/gin"
	"github.com/jpillora/overseer"
	"github.com/uozi-tech/cosy"
)

// RestoreResponse contains the response data for restore operation
type RestoreResponse struct {
	NginxUIRestored bool   `json:"nginx_ui_restored"`
	NginxRestored   bool   `json:"nginx_restored"`
	HashMatch       bool   `json:"hash_match"`
}

// CreateBackup creates a backup of nginx-ui and nginx configurations
// and sends files directly for download
func CreateBackup(c *gin.Context) {
	result, err := backup.Backup()
	if err != nil {
		api.ErrHandler(c, err)
		return
	}

	// Concatenate Key and IV
	securityToken := result.AESKey + ":" + result.AESIv

	// Prepare response content
	reader := bytes.NewReader(result.BackupContent)
	modTime := time.Now()

	// Set HTTP headers for file download
	fileName := result.BackupName
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("X-Backup-Security", securityToken) // Pass security token in header
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	// Send file content
	http.ServeContent(c.Writer, c.Request, fileName, modTime, reader)
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
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrBackupFileNotFound, err.Error()))
		return
	}

	// Validate security token
	if securityToken == "" {
		api.ErrHandler(c, backup.ErrInvalidSecurityToken)
		return
	}

	// Split security token to get Key and IV
	parts := strings.Split(securityToken, ":")
	if len(parts) != 2 {
		api.ErrHandler(c, backup.ErrInvalidSecurityToken)
		return
	}

	aesKey := parts[0]
	aesIv := parts[1]

	// Decode Key and IV from base64
	key, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrInvalidAESKey, err.Error()))
		return
	}

	iv, err := base64.StdEncoding.DecodeString(aesIv)
	if err != nil {
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrInvalidAESIV, err.Error()))
		return
	}

	// Create temporary directory for files
	tempDir, err := os.MkdirTemp("", "nginx-ui-restore-upload-*")
	if err != nil {
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateTempDir, err.Error()))
		return
	}
	defer os.RemoveAll(tempDir)

	// Save backup file
	backupPath := filepath.Join(tempDir, backupFile.Filename)
	if err := c.SaveUploadedFile(backupFile, backupPath); err != nil {
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateBackupFile, err.Error()))
		return
	}

	// Create temporary directory for restore operation
	restoreDir, err := os.MkdirTemp("", "nginx-ui-restore-*")
	if err != nil {
		api.ErrHandler(c, cosy.WrapErrorWithParams(backup.ErrCreateRestoreDir, err.Error()))
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
		api.ErrHandler(c, err)
		return
	}

	// If not actually restoring anything, clean up directory to avoid disk space waste
	if !restoreNginx && !restoreNginxUI {
		defer os.RemoveAll(restoreDir)
	}

	if restoreNginxUI {
		go func() {
			time.Sleep(3 * time.Second)
			// gracefully restart
			overseer.Restart()
		}()
	}

	c.JSON(http.StatusOK, RestoreResponse{
		NginxUIRestored: result.NginxUIRestored,
		NginxRestored:   result.NginxRestored,
		HashMatch:       result.HashMatch,
	})
}
