package site

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy/logger"
)

// GetSiteStatus returns the status of the site
func GetSiteStatus(name string) Status {
	enabledFilePath := nginx.GetConfSymlinkPath(nginx.GetConfPath("sites-enabled", name))
	enabledExists := helper.FileExists(enabledFilePath)
	if enabledExists {
		return StatusEnabled
	}

	mantainanceFilePath := nginx.GetConfPath("sites-enabled", name+MaintenanceSuffix)
	maintenanceExists := helper.FileExists(mantainanceFilePath)
	if maintenanceExists {
		return StatusMaintenance
	}

	logger.Debugf(
		"Site %s considered disabled (enabledPath=%s exists=%t, maintenancePath=%s exists=%t)",
		name, enabledFilePath, enabledExists, mantainanceFilePath, maintenanceExists,
	)
	return StatusDisabled
}
