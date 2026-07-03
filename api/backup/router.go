package backup

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("/backup", middleware.AuthRequired(), middleware.RequireSecureSession(), CreateBackup)
	r.POST("/restore", middleware.AuthRequired(), middleware.RequireSecureSession(), middleware.EncryptedForm(), RestoreBackup)
}

func InitSetupRouter(r *gin.RouterGroup) {
	r.POST("restore", middleware.EncryptedForm(), RestoreBackup)
}

func InitAutoBackupRouter(r *gin.RouterGroup) {
	r.GET("/auto_backup", GetAutoBackupList)
	r.GET("/auto_backup/:id", GetAutoBackup)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("/auto_backup", CreateAutoBackup)
		o.POST("/auto_backup/:id", ModifyAutoBackup)
		o.DELETE("/auto_backup/:id", DestroyAutoBackup)
		o.PATCH("/auto_backup/:id", RestoreAutoBackup)
		o.POST("/auto_backup/test_s3", TestS3Connection)
	}
}
