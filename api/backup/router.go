package backup

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.GET("/backup", CreateBackup)
	r.POST("/restore", middleware.EncryptedForm(), RestoreBackup)
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
