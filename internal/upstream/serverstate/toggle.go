package serverstate

import (
	"context"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/0xJacky/Nginx-UI/internal/stream"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/upstream/managed"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// Request switches one server of one upstream block on or off.
type Request struct {
	Upstream   string `json:"upstream"`
	ConfigPath string `json:"config_path"`
	Address    string `json:"address"`
	Enabled    bool   `json:"enabled"`
}

// knownConfigPath maps the path from the request onto a file the upstream
// scanner (or the managed upstream store) reported for this upstream, so only
// configuration files that really define upstream blocks can be written.
func knownConfigPath(name, requested string) (string, bool) {
	for _, path := range upstream.GetUpstreamService().GetUpstreamConfigPaths() {
		if samePath(path, requested) {
			return path, true
		}
	}
	if managedPath, err := managed.Path(name); err == nil && managed.Exists(name) && samePath(managedPath, requested) {
		return managedPath, true
	}
	return "", false
}

// SetEnabled toggles the `down` parameter of the servers with the requested
// address in the upstream block named req.Upstream, then applies the file the
// same way its own editor does: nginx -t, reload and rollback on failure, plus
// node synchronization. It returns the refreshed upstream block.
func SetEnabled(req Request, userName string) (*Group, error) {
	req.Upstream = strings.TrimSpace(req.Upstream)
	req.Address = strings.TrimSpace(req.Address)
	req.ConfigPath = strings.TrimSpace(req.ConfigPath)
	if req.Upstream == "" || req.Address == "" || req.ConfigPath == "" {
		return nil, ErrInvalidServerToggle
	}

	path, ok := knownConfigPath(req.Upstream, req.ConfigPath)
	if !ok {
		return nil, cosy.WrapErrorWithParams(ErrUpstreamNotInConfig, req.Upstream, req.ConfigPath)
	}
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		return nil, cosy.WrapErrorWithParams(config.ErrPathIsNotUnderTheNginxConfDir, path, nginx.GetConfPath())
	}

	source, canonical := classify(path)
	down := !req.Enabled

	if source.Type == SourceManaged && source.Name == req.Upstream {
		if err := setManagedDown(req.Upstream, req.Address, down, userName); err != nil {
			return nil, err
		}
		return findGroup(canonical, req.Upstream)
	}

	raw, err := nginx.ReadFile(canonical)
	if err != nil {
		return nil, err
	}
	content, matched, changed := SetDown(string(raw), req.Upstream, req.Address, down)
	if matched == 0 {
		if !hasBlock(string(raw), req.Upstream) {
			return nil, cosy.WrapErrorWithParams(ErrUpstreamNotInConfig, req.Upstream, canonical)
		}
		return nil, cosy.WrapErrorWithParams(ErrServerNotFound, req.Address, req.Upstream)
	}
	if !changed {
		return findGroup(canonical, req.Upstream)
	}

	if err := apply(source, canonical, content, userName); err != nil {
		return nil, err
	}

	// Refresh the availability service right away instead of waiting for the
	// file watcher, so every upstream-aware page sees the new state.
	if err := upstream.ScanConfig(canonical, []byte(content)); err != nil {
		logger.Error(err)
	}
	return findGroup(canonical, req.Upstream)
}

// setManagedDown goes through the managed upstream store so the file keeps the
// exact shape the Upstream Groups editor generates.
func setManagedDown(name, address string, down bool, userName string) error {
	u, _, _, err := managed.Read(name)
	if err != nil {
		return err
	}
	matched, changed := 0, false
	for i := range u.Servers {
		if u.Servers[i].Address != address {
			continue
		}
		matched++
		if u.Servers[i].Down != down {
			u.Servers[i].Down = down
			changed = true
		}
	}
	if matched == 0 {
		return cosy.WrapErrorWithParams(ErrServerNotFound, address, name)
	}
	if !changed {
		return nil
	}
	_, err = managed.Save(u, false, userName)
	return err
}

// apply saves content through the path the file's own editor uses.
func apply(source Source, path, content, userName string) error {
	switch source.Type {
	case SourceSite:
		// Keep the namespace and sync targets: site.Save stores what it is given.
		record := &model.Site{}
		if availablePath, err := site.ResolveAvailablePath(source.Name); err == nil {
			s := query.Site
			if found, err := s.Where(s.Path.Eq(availablePath)).Limit(1).Find(); err == nil && len(found) > 0 {
				record = found[0]
			}
		}
		return site.Save(context.Background(), source.Name, content, true, record.NamespaceID, record.SyncNodeIDs,
			model.PostSyncActionReloadNginx)
	case SourceStream:
		record := &model.Stream{}
		if availablePath, err := stream.ResolveAvailablePath(source.Name); err == nil {
			s := query.Stream
			if found, err := s.Where(s.Path.Eq(availablePath)).Limit(1).Find(); err == nil && len(found) > 0 {
				record = found[0]
			}
		}
		return stream.Save(context.Background(), source.Name, content, true, record.SyncNodeIDs, model.PostSyncActionReloadNginx)
	default:
		return config.Save(context.Background(), path, content, nil, userName)
	}
}

func hasBlock(content, name string) bool {
	for _, block := range ParseBlocks(content) {
		if block.Name == name {
			return true
		}
	}
	return false
}

// findGroup re-reads path and returns the upstream block called name.
func findGroup(path, name string) (*Group, error) {
	groups, err := readGroups(path)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].Name == name {
			return &groups[i], nil
		}
	}
	return nil, cosy.WrapErrorWithParams(ErrUpstreamNotInConfig, name, path)
}
