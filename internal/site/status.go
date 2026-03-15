package site

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy/logger"
)

// GetSiteStatus returns the status of the site
func GetSiteStatus(name string) Status {
	enabledFilePath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}

	enabledExists := helper.FileExists(enabledFilePath)
	if enabledExists {
		return StatusEnabled
	}

	mantainanceFilePath, err := ResolveEnabledPath(name + MaintenanceSuffix)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}

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
