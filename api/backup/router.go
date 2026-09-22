package backup

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	// A backup archive carries the crypto secret and every stored credential,
	// and a restore can rewrite app.ini wholesale. Both stay closed in demo mode.
	r.GET("/backup", middleware.AuthRequired(), middleware.RequireSecureSession(), middleware.RejectInDemo(), CreateBackup)
	r.POST("/restore", middleware.AuthRequired(), middleware.RequireSecureSession(), middleware.RejectInDemo(), middleware.EncryptedForm(), RestoreBackup)
}

func InitSetupRouter(r *gin.RouterGroup) {
	r.POST("restore", middleware.EncryptedForm(), RestoreBackup)
}

func InitAutoBackupRouter(r *gin.RouterGroup) {
	r.GET("/auto_backup", GetAutoBackupList)
	// Storage types offered next to the built-in local and S3 storage, with
	// the schema of their configuration form.
	r.GET("/auto_backup/storage_backends", GetStorageBackends)
	r.GET("/auto_backup/:id", GetAutoBackup)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("/auto_backup", CreateAutoBackup)
		o.POST("/auto_backup/:id", ModifyAutoBackup)
		o.DELETE("/auto_backup/:id", DestroyAutoBackup)
		o.PATCH("/auto_backup/:id", middleware.RejectInDemo(), RestoreAutoBackup)
		// Running a job and testing S3 both reach an external endpoint.
		o.POST("/auto_backup/:id/run", middleware.RejectInDemo(), RunAutoBackup)
		o.POST("/auto_backup/test_s3", middleware.RejectInDemo(), TestS3Connection)
		// The runs a plugin storage backend keeps. Listing and testing reach
		// the backend; deleting and restoring change data for good.
		o.POST("/auto_backup/test_storage", middleware.RejectInDemo(), TestPluginStorage)
		o.GET("/auto_backup/:id/stored", middleware.RejectInDemo(), ListStoredBackups)
		o.DELETE("/auto_backup/:id/stored", middleware.RequireInteractiveUser(), middleware.RejectInDemo(), DeleteStoredBackup)
		o.POST("/auto_backup/:id/stored/restore", middleware.RequireInteractiveUser(), middleware.RejectInDemo(), RestoreStoredBackup)
	}
}
