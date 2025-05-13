package site

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// GetSiteStatus returns the status of the site
func GetSiteStatus(name string) SiteStatus {
	enabledFilePath := nginx.GetConfSymlinkPath(nginx.GetConfPath("sites-enabled", name))
	if helper.FileExists(enabledFilePath) {
		return SiteStatusEnabled
	}

	mantainanceFilePath := nginx.GetConfPath("sites-enabled", name+MaintenanceSuffix)
	if helper.FileExists(mantainanceFilePath) {
		return SiteStatusMaintenance
	}

	return SiteStatusDisabled
}
