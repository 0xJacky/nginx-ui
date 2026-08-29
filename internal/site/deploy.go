package site

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// ResolveNamespace returns the namespace owning the site, or nil when the site
// is not tracked in the database or has no namespace.
func ResolveNamespace(name string) *model.Namespace {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		logger.Error(err)
		return nil
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).First()
	if err != nil {
		return nil
	}

	return siteModel.Namespace
}

// ResolveNamespaceByID returns the namespace with the given id, or nil when the
// id is zero or unknown.
func ResolveNamespaceByID(namespaceID uint64) *model.Namespace {
	if namespaceID == 0 {
		return nil
	}

	n := query.Namespace
	namespace, err := n.Where(n.ID.Eq(namespaceID)).First()
	if err != nil {
		return nil
	}

	return namespace
}

// IsRemoteDeploy reports whether the site belongs to a namespace that only
// deploys to remote nodes. Such sites never touch the local Nginx instance:
// no sites-enabled symlink, no local configuration test and no local reload.
func IsRemoteDeploy(name string) bool {
	return ResolveNamespace(name).IsRemoteDeploy()
}

// setRemoteEnabled persists the deployment intent for a remote-deploy site.
func setRemoteEnabled(name string, enabled bool) error {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	s := query.Site
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.RemoteEnabled).
		Updates(&model.Site{RemoteEnabled: enabled})
	return err
}

// remoteStatus maps the persisted deployment intent onto the shared status enum.
func remoteStatus(enabled bool) Status {
	if enabled {
		return StatusEnabled
	}
	return StatusDisabled
}

// detachFromLocalNginx drops the local sites-enabled entries of a site that is
// now deployed remotely. Keeping them would make every local `nginx -t` validate
// configuration meant for other nodes, which is exactly what breaks when two
// namespaces define the same default server.
func detachFromLocalNginx(name string) error {
	enabledPath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	maintenancePath, err := resolveEnabledMaintenancePath(name)
	if err != nil {
		return err
	}

	detached := false
	for _, path := range []string{enabledPath, maintenancePath} {
		if !helper.FileExists(path) {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		detached = true
	}

	if !detached {
		return nil
	}

	// The site was serving locally, so keep the deployment intent and drop it
	// from the running local instance.
	if err := setRemoteEnabled(name, true); err != nil {
		return err
	}

	if res := nginx.Control(nginx.Reload); res.IsError() {
		logger.Warnf("reload after detaching %s from local nginx: %v", name, res.GetError())
	}

	return nil
}

// IsDeployed reports whether the site should be enabled on the sync nodes.
// Remote namespaces keep that intent in the database, local ones in the
// sites-enabled directory.
func IsDeployed(name string) bool {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return false
	}

	s := query.Site
	siteModel, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).First()
	if err == nil && siteModel.Namespace.IsRemoteDeploy() {
		return siteModel.RemoteEnabled
	}

	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return false
	}

	return helper.FileExists(enabledPath)
}
