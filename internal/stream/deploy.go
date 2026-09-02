package stream

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// ResolveNamespace returns the namespace owning the stream, or nil when the
// stream is not tracked in the database or has no namespace.
func ResolveNamespace(name string) *model.Namespace {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		logger.Error(err)
		return nil
	}

	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).First()
	if err != nil {
		return nil
	}

	return streamModel.Namespace
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

// IsRemoteDeploy reports whether the stream belongs to a namespace that only
// deploys to remote nodes. Such streams never touch the local Nginx instance.
func IsRemoteDeploy(name string) bool {
	return ResolveNamespace(name).IsRemoteDeploy()
}

// setRemoteEnabled persists the deployment intent for a remote-deploy stream.
func setRemoteEnabled(name string, enabled bool) error {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return err
	}

	s := query.Stream
	_, err = s.Where(s.Path.Eq(path)).
		Select(s.RemoteEnabled).
		Updates(&model.Stream{RemoteEnabled: enabled})
	return err
}

// remoteStatus maps the persisted deployment intent onto the shared status enum.
func remoteStatus(enabled bool) config.Status {
	if enabled {
		return config.StatusEnabled
	}
	return config.StatusDisabled
}

// detachFromLocalNginx drops the local streams-enabled symlink of a stream that
// is now deployed remotely, so local configuration tests stop validating it.
func detachFromLocalNginx(name string) error {
	enabledPath, err := resolveEnabledSymlinkPath(name)
	if err != nil {
		return err
	}

	enabledExists, err := nginx.Exists(enabledPath)
	if err != nil {
		return err
	}
	if !enabledExists {
		return nil
	}

	if err := nginx.Remove(enabledPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := setRemoteEnabled(name, true); err != nil {
		return err
	}

	if res := nginx.Control(nginx.Reload); res.IsError() {
		logger.Warnf("reload after detaching %s from local nginx: %v", name, res.GetError())
	}

	return nil
}

// IsDeployed reports whether the stream should be enabled on the sync nodes.
// Remote namespaces keep that intent in the database, local ones in the
// streams-enabled directory.
func IsDeployed(name string) bool {
	path, err := ResolveAvailablePath(name)
	if err != nil {
		return false
	}

	s := query.Stream
	streamModel, err := s.Where(s.Path.Eq(path)).Preload(s.Namespace).First()
	if err == nil && streamModel.Namespace.IsRemoteDeploy() {
		return streamModel.RemoteEnabled
	}

	enabledPath, err := ResolveEnabledPath(name)
	if err != nil {
		return false
	}

	enabledExists, err := nginx.Exists(enabledPath)
	if err != nil {
		logger.Error(err)
		return false
	}

	return enabledExists
}
