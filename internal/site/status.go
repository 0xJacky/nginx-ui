package site

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// GetSiteStatus returns the status of the site
func GetSiteStatus(name string) Status {
	// Remote namespaces never create a local symlink, their state lives in the
	// database instead.
	if path, err := ResolveAvailablePath(name); err == nil {
		s := query.Site
		siteModels, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).Find()
		if err == nil && len(siteModels) > 0 && siteModels[0].Namespace.IsRemoteDeploy() {
			return remoteStatus(siteModels[0].RemoteEnabled)
		}
	}

	enabledFilePath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}

	enabledExists, err := nginx.Exists(enabledFilePath)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}
	if enabledExists {
		return StatusEnabled
	}

	mantainanceFilePath, err := ResolveEnabledMaintenancePath(name)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}

	maintenanceExists, err := nginx.Exists(mantainanceFilePath)
	if err != nil {
		logger.Error(err)
		return StatusDisabled
	}
	if maintenanceExists {
		return StatusMaintenance
	}

	logger.Debugf(
		"Site %s considered disabled (enabledPath=%s exists=%t, maintenancePath=%s exists=%t)",
		name, enabledFilePath, enabledExists, mantainanceFilePath, maintenanceExists,
	)
	return StatusDisabled
}
