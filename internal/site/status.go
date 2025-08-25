package site

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// GetSiteStatus returns the status of the site
func GetSiteStatus(name string) Status {
	enabledFilePath := nginx.GetConfSymlinkPath(nginx.GetConfPath("sites-enabled", name))
	if helper.FileExists(enabledFilePath) {
		return StatusEnabled
	}

	mantainanceFilePath := nginx.GetConfPath("sites-enabled", name+MaintenanceSuffix)
	if helper.FileExists(mantainanceFilePath) {
		return StatusMaintenance
	}

	return StatusDisabled
}
