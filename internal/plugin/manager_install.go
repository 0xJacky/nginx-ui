package plugin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// Plugin change actions published on the event bus.
const (
	ActionInstalled   = "installed"
	ActionUpgraded    = "upgraded"
	ActionUninstalled = "uninstalled"
	ActionEnabled     = "enabled"
	ActionDisabled    = "disabled"
)

// Inspect reads a package without installing it, so the UI can show what the
// user is about to trust.
func (m *Manager) Inspect(archivePath string) (*InspectResult, error) {
	staging, err := os.MkdirTemp("", "nginx-ui-plugin-inspect-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	manifest, err := ExtractPackage(archivePath, filepath.Join(staging, "payload"))
	if err != nil {
		return nil, err
	}

	result := &InspectResult{
		Manifest:        manifest,
		Permissions:     manifest.Permissions,
		RequiresMissing: m.missingRequirements(manifest),
	}
	if result.Permissions == nil {
		result.Permissions = []string{}
	}

	if item, ok := m.lookup(manifest.ID); ok {
		m.mu.RLock()
		if item.manifest != nil {
			result.InstalledVersion = item.manifest.Version
		}
		if item.row != nil && item.row.ApprovedPermissionsHash != "" {
			result.PermissionsChanged = item.row.ApprovedPermissionsHash != PermissionsHash(manifest)
		}
		m.mu.RUnlock()
	}
	return result, nil
}

// Install unpacks a package into the plugin directory, creating or upgrading
// the plugin. The database row, its settings and the key value store survive
// an upgrade.
func (m *Manager) Install(ctx context.Context, archivePath string, opts InstallOptions) (*Info, error) {
	if !settings.PluginSettings.Enabled {
		return nil, ErrPluginsDisabled
	}

	m.opMu.Lock()
	defer m.opMu.Unlock()

	if err := os.MkdirAll(m.Dir(), 0o755); err != nil {
		return nil, err
	}
	// Staging lives next to the plugins so the final move stays on one volume.
	staging, err := os.MkdirTemp(m.Dir(), ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, "payload")
	manifest, err := ExtractPackage(archivePath, staged)
	if err != nil {
		return nil, err
	}
	if !IsCompatible(manifest) {
		return nil, ErrIncompatibleAPIVersion
	}
	if err = checkHostVersion(manifest); err != nil {
		return nil, err
	}
	if missing := m.missingRequirements(manifest); len(missing) > 0 {
		return nil, cosy.WrapErrorWithParams(ErrDependencyMissing, requirementList(missing))
	}

	item, upgrading := m.lookup(manifest.ID)
	if upgrading {
		m.mu.RLock()
		upgrading = item.manifest != nil
		m.mu.RUnlock()
	}
	if upgrading {
		m.dropSupervisor(ctx, item)
		m.stopEventPump(item)
	}

	target := filepath.Join(m.Dir(), manifest.ID)
	restore, err := moveIntoPlace(staged, target)
	if err != nil {
		return nil, err
	}

	info, err := m.finishInstall(ctx, manifest, opts, upgrading)
	if err != nil {
		restore(false)
		return nil, err
	}
	restore(true)
	// Cluster sync pushes the very bytes this node was given, so the package
	// is kept next to the plugins instead of being rebuilt from the files.
	m.keepArchive(manifest.ID, manifest.Version, archivePath)
	return info, nil
}

// moveIntoPlace swaps a staged directory in, keeping the previous version as
// <id>.bak. The returned function commits or rolls the move back.
func moveIntoPlace(staged, target string) (func(commit bool), error) {
	backup := target + backupSuffix
	_ = os.RemoveAll(backup)

	hadPrevious := false
	if _, err := os.Stat(target); err == nil {
		if err = os.Rename(target, backup); err != nil {
			return nil, err
		}
		hadPrevious = true
	}

	if err := os.Rename(staged, target); err != nil {
		if hadPrevious {
			_ = os.Rename(backup, target)
		}
		return nil, err
	}

	return func(commit bool) {
		if commit {
			_ = os.RemoveAll(backup)
			return
		}
		_ = os.RemoveAll(target)
		if hadPrevious {
			_ = os.Rename(backup, target)
		}
	}, nil
}

// finishInstall updates the database row and brings the plugin up.
func (m *Manager) finishInstall(ctx context.Context, manifest *protocol.Manifest, opts InstallOptions, upgrading bool) (*Info, error) {
	row, err := m.loadOrCreateRow(ctx, manifest.ID)
	if err != nil {
		return nil, err
	}

	hash := PermissionsHash(manifest)
	approved := row.ApprovedPermissionsHash == hash || len(manifest.Permissions) == 0
	switch {
	case approved, opts.ApprovePermissions:
		// Either nothing changed or the caller confirmed the new set.
		row.ApprovedPermissionsHash = hash
		approved = true
	case row.ApprovedPermissionsHash == "":
		// A first install is approved by the act of installing it: the user
		// saw the permission list before uploading the package.
		row.ApprovedPermissionsHash = hash
		approved = true
	}

	row.Version = manifest.Version
	row.LastError = ""
	if row.SyncPolicy == "" {
		row.SyncPolicy = settings.PluginSettings.GetDefaultSyncPolicy()
	}
	if opts.Enable {
		row.Enabled = true
	}
	if !approved {
		// An upgrade that asks for more stays down until it is approved.
		row.Enabled = false
	}
	if err = m.saveRow(ctx, row); err != nil {
		return nil, err
	}

	item := m.newEntry(manifest.ID, manifest, row)
	m.mu.Lock()
	if previous, ok := m.entries[manifest.ID]; ok {
		item.dropped.Store(previous.dropped.Load())
	}
	m.entries[manifest.ID] = item
	offline := m.offline
	m.mu.Unlock()
	if !offline {
		m.startEventPump(item)
	}

	m.mu.RLock()
	ready := runnableLocked(item)
	m.mu.RUnlock()
	if ready {
		if err = m.bringUp(ctx, item); err != nil {
			m.log.Errorf("[plugin:%s] start after install: %v", manifest.ID, err)
			m.recordError(item, err)
		}
	}

	action := ActionInstalled
	if upgrading {
		action = ActionUpgraded
	}
	publishChange(manifest.ID, action)

	info := m.infoOf(item)
	return &info, nil
}

// loadOrCreateRow returns the database row of a plugin, creating it when the
// package is new to this node.
func (m *Manager) loadOrCreateRow(ctx context.Context, id string) (*model.Plugin, error) {
	row, err := query.Plugin.WithContext(ctx).Where(query.Plugin.PluginID.Eq(id)).First()
	if err == nil {
		return row, nil
	}

	row = &model.Plugin{
		PluginID:   id,
		SyncPolicy: settings.PluginSettings.GetDefaultSyncPolicy(),
	}
	if err = query.Plugin.WithContext(ctx).Create(row); err != nil {
		return nil, err
	}
	return row, nil
}

// Uninstall stops a plugin and removes its files, its data directory, its row
// and its key value store. Cascade also uninstalls whatever depends on it.
func (m *Manager) Uninstall(ctx context.Context, id string, cascade bool) error {
	if !settings.PluginSettings.Enabled {
		return ErrPluginsDisabled
	}

	m.opMu.Lock()
	defer m.opMu.Unlock()
	return m.uninstall(ctx, id, cascade, map[string]bool{})
}

func (m *Manager) uninstall(ctx context.Context, id string, cascade bool, seen map[string]bool) error {
	if seen[id] {
		return nil
	}
	seen[id] = true

	item, ok := m.lookup(id)
	if !ok {
		return ErrPluginNotFound
	}

	if dependents := m.dependents(id); len(dependents) > 0 {
		if !cascade {
			return cosy.WrapErrorWithParams(ErrPluginInUse, strings.Join(dependents, ", "))
		}
		for _, dependent := range dependents {
			if err := m.uninstall(ctx, dependent, cascade, seen); err != nil {
				return err
			}
		}
	}

	m.dropSupervisor(ctx, item)
	m.stopEventPump(item)

	m.mu.RLock()
	dir, dataDir := item.dir, item.dataDir
	row := item.row
	m.mu.RUnlock()

	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.RemoveAll(dataDir); err != nil {
		m.log.Warnf("[plugin:%s] remove data directory: %v", id, err)
	}
	// Both tables carry a unique index on the plugin id, so the rows are
	// deleted for good and a later reinstall can create them again.
	if _, err := query.PluginKV.WithContext(ctx).Unscoped().Where(query.PluginKV.PluginID.Eq(id)).Delete(); err != nil {
		m.log.Warnf("[plugin:%s] remove key value store: %v", id, err)
	}
	if row != nil {
		if _, err := query.Plugin.WithContext(ctx).Unscoped().Where(query.Plugin.ID.Eq(row.ID)).Delete(); err != nil {
			return err
		}
	}

	m.mu.Lock()
	delete(m.entries, id)
	m.mu.Unlock()

	publishChange(id, ActionUninstalled)
	return nil
}

// Enable marks a plugin as wanted and starts it. A permission set the user has
// not approved yet blocks the start and leaves the plugin in needs_approval.
func (m *Manager) Enable(ctx context.Context, id string, approvePermissions bool) (*Info, error) {
	if !settings.PluginSettings.Enabled {
		return nil, ErrPluginsDisabled
	}

	m.opMu.Lock()
	defer m.opMu.Unlock()

	item, ok := m.lookup(id)
	if !ok {
		return nil, ErrPluginNotFound
	}

	m.mu.RLock()
	manifest := item.manifest
	row := item.row
	approvedBefore := ""
	if row != nil {
		approvedBefore = row.ApprovedPermissionsHash
	}
	m.mu.RUnlock()

	if manifest == nil || row == nil {
		return nil, ErrPluginNotFound
	}
	if !IsCompatible(manifest) {
		return nil, ErrIncompatibleAPIVersion
	}
	if err := checkHostVersion(manifest); err != nil {
		return nil, err
	}
	if missing := m.missingRequirements(manifest); len(missing) > 0 {
		return nil, cosy.WrapErrorWithParams(ErrDependencyMissing, requirementList(missing))
	}

	hash := PermissionsHash(manifest)
	switch {
	case len(manifest.Permissions) == 0, approvedBefore == hash:
		// Nothing new to approve.
	case approvePermissions, approvedBefore == "":
		// Either the user confirmed the dialog or this plugin was never
		// enabled before, in which case turning it on is the approval.
		m.mu.Lock()
		row.ApprovedPermissionsHash = hash
		m.mu.Unlock()
	default:
		return nil, ErrPermissionApprovalRequired
	}

	m.mu.Lock()
	row.Enabled = true
	row.LastError = ""
	item.lastErr = ""
	m.mu.Unlock()
	if err := m.saveRow(ctx, row); err != nil {
		return nil, err
	}

	// A fresh supervisor picks up the approved permissions and the settings.
	m.dropSupervisor(ctx, item)
	if err := m.bringUp(ctx, item); err != nil {
		m.recordError(item, err)
		return nil, err
	}

	publishChange(id, ActionEnabled)
	info := m.infoOf(item)
	return &info, nil
}

// Disable stops a plugin and everything that requires it.
func (m *Manager) Disable(ctx context.Context, id string) (*Info, error) {
	if !settings.PluginSettings.Enabled {
		return nil, ErrPluginsDisabled
	}

	m.opMu.Lock()
	defer m.opMu.Unlock()

	item, ok := m.lookup(id)
	if !ok {
		return nil, ErrPluginNotFound
	}

	// Whatever depends on this plugin cannot keep running without it.
	for _, dependent := range m.dependents(id) {
		other, found := m.lookup(dependent)
		if !found {
			continue
		}
		m.stopEntry(ctx, other)
		m.recordError(other, cosy.WrapErrorWithParams(ErrDependencyMissing, id))
	}

	m.stopEntry(ctx, item)

	m.mu.Lock()
	row := item.row
	if row != nil {
		row.Enabled = false
		row.LastError = ""
	}
	item.lastErr = ""
	m.mu.Unlock()

	if row != nil {
		if err := m.saveRow(ctx, row); err != nil {
			return nil, err
		}
	}

	publishChange(id, ActionDisabled)
	info := m.infoOf(item)
	return &info, nil
}

// requirementList renders a dependency list for an error message.
func requirementList(requirements []protocol.ManifestRequirement) string {
	parts := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.Version == "" {
			parts = append(parts, requirement.ID)
			continue
		}
		parts = append(parts, requirement.ID+"@"+requirement.Version)
	}
	slices.Sort(parts)
	return strings.Join(parts, ", ")
}

// publishChange tells the rest of the node, the cluster and the subscribed
// plugins that the plugin inventory moved.
func publishChange(id, action string) {
	event.Publish(event.Event{
		Type: event.TypePluginChanged,
		Data: map[string]any{"plugin_id": id, "action": action},
	})
}
