package backup

import (
	"github.com/0xJacky/Nginx-UI/api/system"
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

// authIfInstalled requires auth if system is installed
func authIfInstalled(ctx *gin.Context) {
	if system.InstallLockStatus() || system.IsInstallTimeoutExceeded() {
		middleware.AuthRequired()(ctx)
	} else {
		ctx.Next()
	}
}

func InitRouter(r *gin.RouterGroup) {
	// Backup always requires authentication (contains sensitive data)
	r.GET("/backup", middleware.AuthRequired(), CreateBackup)

	// Restore requires auth only after installation
	// This allows restoring backup during initial setup
	r.POST("/restore", authIfInstalled, middleware.EncryptedForm(), RestoreBackup)
}

func InitAutoBackupRouter(r *gin.RouterGroup) {
	r.GET("/auto_backup", GetAutoBackupList)
	r.POST("/auto_backup", CreateAutoBackup)
	r.GET("/auto_backup/:id", GetAutoBackup)
	r.POST("/auto_backup/:id", ModifyAutoBackup)
	r.DELETE("/auto_backup/:id", DestroyAutoBackup)
	r.PATCH("/auto_backup/:id", RestoreAutoBackup)
	r.POST("/auto_backup/test_s3", TestS3Connection)
}
