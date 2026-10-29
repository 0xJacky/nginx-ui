package config

import (
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"gorm.io/gorm"
)

func GetConfigHistory(c *gin.Context) {
	// SetEqual("filepath") compares the raw query value against the stored
	// column, which stops matching once the client sends the path base64url
	// encoded to get it past a WAF. Decode first, then filter on the real path.
	core := cosy.Core[model.ConfigBackup](c)

	if filepath, _ := helper.DecodePathParam(c.Query("filepath")); filepath != "" {
		core = core.GormScope(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("filepath = ?", filepath)
		})
	}

	// History keeps earlier revisions of every file saved through the editor,
	// including secret files, so read-only service tokens get the same
	// redaction as a direct file read.
	if internalmcp.IsReadOnlyServiceTokenRequest(c) {
		core.SetTransformer(config.SecretBackupRedactor(config.SecretFileReferences()))
	}

	core.PagingList()
}
