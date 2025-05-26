package backup

import (
	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.RouterGroup) {
	r.POST("/backup", CreateBackup)
	r.POST("/restore", RestoreBackup)
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
