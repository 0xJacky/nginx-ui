package system

import (
	"bytes"
	"net/http"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/backup"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// CreateBackup creates a backup of nginx-ui and nginx configurations
// and sends files directly for download
func CreateBackup(c *gin.Context) {
	result, err := backup.Backup()
	if err != nil {
		cosy.ErrHandler(c, err)
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
