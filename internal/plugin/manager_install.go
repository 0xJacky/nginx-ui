package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
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

	payload := filepath.Join(staging, "payload")
	manifest, err := ExtractPackage(archivePath, payload)
	if err != nil {
		return nil, err
	}

	trust, err := verifyPackageSignature(payload, "", nil, m.partnerKeyring())
	if err != nil {
		return nil, err
	}

	platforms := packagePlatforms(manifest, payload)
	result := &InspectResult{
		Manifest:          manifest,
		Permissions:       manifest.Permissions,
		RequiresMissing:   m.missingRequirements(manifest),
		Platforms:         platforms,
		HostPlatform:      HostPlatform(),
		PlatformSupported: platformsCover(platforms, HostPlatform()),
		Trust:             trust.Trust,
		Signer:            trust.Signer,
		Partner:           trust.Partner,
	}
	if signer, signature, err := readSignerFiles(payload); err == nil && signer != nil {
		if certificate, err := parseSignerCertificate(payload, signer, signature); err == nil {
			result.CertifiedSigner = certificate.KeyID
		}
	}
	if result.Permissions == nil {
		result.Permissions = []string{}
	}
	result.NameI18n, result.DescriptionI18n = i18nMaps(manifest)

	if item, ok := m.lookup(manifest.ID); ok {
		m.mu.RLock()
		if item.manifest != nil {
			result.InstalledVersion = item.manifest.Version
		}
		if item.row != nil && item.row.ApprovedPermissions != nil {
			result.PermissionsChanged = len(unapprovedPermissions(item.row.ApprovedPermissions, manifest)) > 0
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
	// Unpacking and hashing leave temporary memory behind that a small host
	// should give back right away.
	defer debug.FreeOSMemory()

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
	if err = checkExpected(manifest, opts); err != nil {
		return nil, err
	}
	// Nothing is moved before the signature is verified and the policy has
	// accepted the trust it proves.
	trust, err := checkPackageTrust(staged, opts, m.partnerKeyring())
	if err != nil {
		return nil, err
	}
	// Whatever catalog or file a package comes from, only the official key
	// can publish in the official namespace.
	if err = checkReservedID(manifest.ID, trust.Trust); err != nil {
		return nil, err
	}
	if !IsCompatible(manifest) {
		return nil, ErrIncompatibleAPIVersion
	}
	if err = checkHostVersion(manifest); err != nil {
		return nil, err
	}
	// A per-platform package built for another node would install fine and
	// then never start, so it is refused up front.
	if !platformsCover(packagePlatforms(manifest, staged), HostPlatform()) {
		return nil, ErrNoExecutableForPlatform
	}
	// Templates and translation files are read like the built-in ones, so a
	// file that does not parse is refused before it is installed.
	if err = validateContentFiles(manifest, staged); err != nil {
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

	info, err := m.finishInstall(ctx, manifest, trust, opts, upgrading)
	if err != nil {
		restore(false)
		if upgrading {
			m.resume(ctx, item)
		}
		return nil, err
	}
	restore(true)
	// Cluster sync pushes the very bytes this node was given, so the package
	// is kept next to the plugins instead of being rebuilt from the files.
	m.keepArchive(manifest.ID, manifest.Version, archivePath)
	return info, nil
}

// checkExpected refuses a package that is not what the caller was promised.
func checkExpected(manifest *protocol.Manifest, opts InstallOptions) error {
	if opts.ExpectedID != "" && manifest.ID != opts.ExpectedID {
		return ErrPluginIDMismatch
	}
	if opts.ExpectedVersion != "" && manifest.Version != opts.ExpectedVersion {
		return cosy.WrapErrorWithParams(ErrPluginVersionMismatch, manifest.Version, opts.ExpectedVersion)
	}
	return nil
}

// resume brings the previous version back up after a failed upgrade put its
// files back. The entry never left the inventory, only its process and its
// event queue were taken down for the upgrade.
func (m *Manager) resume(ctx context.Context, item *entry) {
	m.mu.RLock()
	offline := m.offline
	ready := runnableLocked(item)
	m.mu.RUnlock()
	if offline {
		return
	}
	m.startEventPump(item)
	if !ready {
		return
	}
	if err := m.bringUp(ctx, item); err != nil {
		m.log.Errorf("[plugin:%s] restart after a failed upgrade: %v", item.id, err)
		m.recordError(item, err)
	}
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
func (m *Manager) finishInstall(ctx context.Context, manifest *protocol.Manifest, trust packageTrust,
	opts InstallOptions, upgrading bool,
) (*Info, error) {
	row, created, err := m.loadOrCreateRow(ctx, manifest.ID)
	if err != nil {
		return nil, err
	}
	if created {
		seedSettings(manifest, row)
	}

	approved := len(unapprovedPermissions(row.ApprovedPermissions, manifest)) == 0
	switch {
	case approved, opts.ApprovePermissions:
		// Either nothing new is asked for or the caller confirmed the new set.
		// Storing the current set forgets a permission the upgrade dropped, so
		// asking for it again needs a new approval.
		row.ApprovedPermissions = approvedSet(manifest)
		approved = true
	case row.ApprovedPermissions == nil:
		// A first install is approved by the act of installing it: the user
		// saw the permission list before uploading the package.
		row.ApprovedPermissions = approvedSet(manifest)
		approved = true
	}

	row.Version = manifest.Version
	row.Trust = trust.Trust
	row.Signer = trust.Signer
	row.Partner = trust.Partner
	row.AuthorPublicKey = trust.AuthorKey
	// The followed channel stays what the person chose. The release only
	// raises the channel updates come from while it runs, see effectiveChannel.
	row.ReleaseChannel = opts.Channel
	if !IsValidChannel(row.ReleaseChannel) {
		row.ReleaseChannel = InferChannel(manifest.Version)
	}
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
	// A conflict with an enabled plugin keeps this one down, unless the
	// caller asked to replace it. An upgrade can add one.
	var conflicts []string
	if row.Enabled {
		if conflicts = m.enabledConflicts(manifest); len(conflicts) > 0 && !opts.ReplaceConflicts {
			row.Enabled = false
		}
	}
	if err = m.saveRow(ctx, row); err != nil {
		return nil, err
	}
	if row.Enabled {
		if err = m.disableAll(ctx, conflicts); err != nil {
			return nil, err
		}
	}

	item := m.newEntry(manifest.ID, manifest, row)
	m.mu.Lock()
	if previous, ok := m.entries[manifest.ID]; ok {
		item.dropped.Store(previous.dropped.Load())
		item.logCounters.streamed.Store(previous.logCounters.streamed.Load())
		item.logCounters.rejected.Store(previous.logCounters.rejected.Load())
		item.logCounters.dropped.Store(previous.logCounters.dropped.Load())
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
// package is new to this node. created reports which of the two happened.
func (m *Manager) loadOrCreateRow(ctx context.Context, id string) (row *model.Plugin, created bool, err error) {
	row, err = query.Plugin.WithContext(ctx).Where(query.Plugin.PluginID.Eq(id)).First()
	if err == nil {
		return row, false, nil
	}

	row = &model.Plugin{
		PluginID:   id,
		SyncPolicy: settings.PluginSettings.GetDefaultSyncPolicy(),
	}
	if err = query.Plugin.WithContext(ctx).Create(row); err != nil {
		return nil, false, err
	}
	return row, true, nil
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
	if err := m.backend.removeSnippets(id); err != nil {
		m.log.Warnf("[plugin:%s] remove nginx snippets: %v", id, err)
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

// EnableOptions controls one Enable call.
type EnableOptions struct {
	// ApprovePermissions records the manifest permission set as approved.
	ApprovePermissions bool
	// ReplaceConflicts disables the enabled plugins that conflict with the
	// plugin instead of refusing.
	ReplaceConflicts bool
}

// Enable marks a plugin as wanted and starts it. A permission set the user has
// not approved yet blocks the start and leaves the plugin in needs_approval.
func (m *Manager) Enable(ctx context.Context, id string, approvePermissions bool) (*Info, error) {
	return m.EnableWith(ctx, id, EnableOptions{ApprovePermissions: approvePermissions})
}

// EnableWith is Enable with the full set of options. An enabled plugin that
// conflicts with this one refuses the call, unless ReplaceConflicts is set and
// the conflicting plugins are turned off first.
func (m *Manager) EnableWith(ctx context.Context, id string, opts EnableOptions) (*Info, error) {
	approvePermissions := opts.ApprovePermissions
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
	var approvedBefore []string
	if row != nil {
		approvedBefore = row.ApprovedPermissions
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

	conflicts := m.enabledConflicts(manifest)
	if len(conflicts) > 0 && !opts.ReplaceConflicts {
		return nil, cosy.WrapErrorWithParams(ErrPluginConflict, strings.Join(conflicts, ", "))
	}

	switch {
	case len(unapprovedPermissions(approvedBefore, manifest)) == 0:
		// Nothing new to approve.
	case approvePermissions, approvedBefore == nil:
		// Either the user confirmed the dialog or this plugin was never
		// enabled before, in which case turning it on is the approval.
		m.mu.Lock()
		row.ApprovedPermissions = approvedSet(manifest)
		m.mu.Unlock()
	default:
		return nil, ErrPermissionApprovalRequired
	}

	// The approval is settled, so nothing is turned off for a call that fails.
	if err := m.disableAll(ctx, conflicts); err != nil {
		return nil, err
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
	return m.disable(ctx, id)
}

// disableAll turns the given plugins off. The caller must hold opMu.
func (m *Manager) disableAll(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if _, err := m.disable(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// disable is Disable for a caller that holds opMu.
func (m *Manager) disable(ctx context.Context, id string) (*Info, error) {
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
