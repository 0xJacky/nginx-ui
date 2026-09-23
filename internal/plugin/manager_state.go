package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/spf13/cast"
	"github.com/uozi-tech/cosy"
)

// startEnabled brings every enabled plugin up, dependencies first.
func (m *Manager) startEnabled(ctx context.Context) {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	ordered, err := m.startOrder()
	if err != nil {
		// A cycle only breaks the ordering, the plugins themselves still run.
		m.log.Errorf("Plugin dependency order: %v", err)
		ordered = m.sortedIDs()
	}

	for _, id := range ordered {
		item, ok := m.lookup(id)
		if !ok {
			continue
		}
		m.mu.RLock()
		ready := runnableLocked(item)
		m.mu.RUnlock()
		if !ready {
			continue
		}
		if err = m.bringUp(ctx, item); err != nil {
			m.log.Errorf("[plugin:%s] start: %v", id, err)
			m.recordError(item, err)
		}
	}
}

// bringUp prepares the supervisor, registers the manifest cron entries and
// starts a resident plugin. An on_demand plugin is only registered, the first
// Acquire starts it.
func (m *Manager) bringUp(ctx context.Context, item *entry) error {
	m.mu.RLock()
	offline := m.offline
	m.mu.RUnlock()
	if offline {
		return nil
	}

	supervisor, err := m.ensureSupervisor(item)
	if err != nil {
		return err
	}
	if supervisor == nil {
		// A plugin without a server block has no process: there is nothing
		// to start and nothing to schedule (spec CONTENT-1).
		return nil
	}
	m.registerManifestCron(item)
	if supervisor.Lifecycle() == protocol.LifecycleOnDemand {
		return nil
	}
	return supervisor.Start(ctx)
}

// runnableLocked reports whether an enabled plugin may be brought up. The
// caller must hold at least the read lock.
func runnableLocked(item *entry) bool {
	return item.manifest != nil && item.row != nil && item.row.Enabled &&
		IsCompatible(item.manifest) && isApproved(item)
}

// ensureSupervisor lazily builds the supervisor of a plugin. A plugin without
// a server block has nothing to supervise and yields a nil supervisor.
func (m *Manager) ensureSupervisor(item *entry) (*Supervisor, error) {
	m.mu.RLock()
	existing := item.supervisor
	manifest := item.manifest
	row := item.row
	dir, dataDir := item.dir, item.dataDir
	m.mu.RUnlock()

	if existing != nil {
		return existing, nil
	}
	if manifest == nil || manifest.Server == nil {
		return nil, nil
	}

	argv, err := ResolveExecutable(manifest, dir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}

	id := item.id
	permissions := slices.Clone(manifest.Permissions)
	backend := m.hostBackend()
	supervisor := NewSupervisor(SupervisorConfig{
		PluginID:         id,
		Dir:              dir,
		DataDir:          dataDir,
		Manifest:         manifest,
		Argv:             argv,
		HostVersion:      version.GetVersionInfo().Version,
		Locale:           backend.Locale(),
		Settings:         mergedSettings(manifest, row),
		Permissions:      permissions,
		HandshakeTimeout: m.handshakeTimeout,
		HostHandlers: func(conn *jsonrpc.Conn) {
			RegisterHostHandlers(conn, id, permissions, backend)
		},
		OnStateChange: func(state State, cause error) { m.onStateChange(id, state, cause) },
		Logger:        m.log,
	})

	m.mu.Lock()
	item.supervisor = supervisor
	m.mu.Unlock()
	return supervisor, nil
}

// stopEntry stops the process and the cron entries but keeps the supervisor so
// the buffered logs stay visible.
func (m *Manager) stopEntry(ctx context.Context, item *entry) {
	m.unregisterCron(item)

	m.mu.RLock()
	supervisor := item.supervisor
	m.mu.RUnlock()
	if supervisor == nil {
		return
	}
	if err := supervisor.Stop(ctx); err != nil {
		m.log.Warnf("[plugin:%s] stop: %v", item.id, err)
	}
	m.mu.Lock()
	item.state = StateStopped
	m.mu.Unlock()
}

// dropSupervisor stops the plugin and forgets the supervisor, which is what an
// upgrade, an uninstall and a re-enable need so the next start reads fresh
// settings and permissions.
func (m *Manager) dropSupervisor(ctx context.Context, item *entry) {
	m.stopEntry(ctx, item)
	m.mu.Lock()
	item.supervisor = nil
	m.mu.Unlock()
}

// onStateChange mirrors a supervisor transition into the manager and persists
// the failure so the UI still shows it after a restart.
func (m *Manager) onStateChange(id string, state State, cause error) {
	message := ""
	if cause != nil {
		message = cause.Error()
	}

	m.mu.Lock()
	item, ok := m.entries[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	item.state = state
	item.lastErr = message
	row := item.row
	changed := row != nil && row.LastError != message
	if changed {
		row.LastError = message
		rowID := row.ID
		m.mu.Unlock()
		m.persistLastError(rowID, message)
		return
	}
	m.mu.Unlock()
}

// recordError stores a manager level failure on the plugin.
func (m *Manager) recordError(item *entry, cause error) {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	m.mu.Lock()
	item.lastErr = message
	item.state = StateError
	var rowID uint64
	if item.row != nil {
		item.row.LastError = message
		rowID = item.row.ID
	}
	m.mu.Unlock()
	if rowID != 0 {
		m.persistLastError(rowID, message)
	}
}

// saveRow persists a plugin row. The row is shared with the supervisor
// callbacks, so it is copied under the lock before gorm reflects over it.
func (m *Manager) saveRow(ctx context.Context, row *model.Plugin) error {
	m.mu.RLock()
	snapshot := *row
	m.mu.RUnlock()

	if err := query.Plugin.WithContext(ctx).Save(&snapshot); err != nil {
		return err
	}

	m.mu.Lock()
	row.ID = snapshot.ID
	row.CreatedAt = snapshot.CreatedAt
	row.UpdatedAt = snapshot.UpdatedAt
	m.mu.Unlock()
	return nil
}

func (m *Manager) persistLastError(rowID uint64, message string) {
	if rowID == 0 {
		return
	}
	_, err := query.Plugin.WithContext(context.Background()).
		Where(query.Plugin.ID.Eq(rowID)).
		Update(query.Plugin.LastError, message)
	if err != nil {
		m.log.Warnf("Persist plugin error state: %v", err)
	}
}

// sortedIDs lists every known plugin id in a stable order.
func (m *Manager) sortedIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.entries))
	for id := range m.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// startOrder topologically sorts the installed plugins so a dependency always
// starts before the plugin that requires it.
func (m *Manager) startOrder() ([]string, error) {
	m.mu.RLock()
	ids := make([]string, 0, len(m.entries))
	deps := make(map[string][]string, len(m.entries))
	for id, item := range m.entries {
		ids = append(ids, id)
		if item.manifest == nil {
			continue
		}
		for _, requirement := range item.manifest.Requires {
			if _, ok := m.entries[requirement.ID]; ok {
				deps[id] = append(deps[id], requirement.ID)
			}
		}
	}
	m.mu.RUnlock()

	sort.Strings(ids)
	return topoSort(ids, deps)
}

// topoSort orders ids so that every dependency precedes its dependents.
func topoSort(ids []string, deps map[string][]string) ([]string, error) {
	const (
		unvisited = iota
		visiting
		visited
	)
	mark := make(map[string]int, len(ids))
	ordered := make([]string, 0, len(ids))

	var visit func(string) error
	visit = func(id string) error {
		switch mark[id] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("%w: %s", ErrDependencyCycle, id)
		}
		mark[id] = visiting
		for _, dep := range deps[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		mark[id] = visited
		ordered = append(ordered, id)
		return nil
	}

	for _, id := range ids {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// dependents lists the installed plugins that require the given plugin.
func (m *Manager) dependents(id string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dependents := make([]string, 0, len(m.entries))
	for other, item := range m.entries {
		if other == id || item.manifest == nil {
			continue
		}
		for _, requirement := range item.manifest.Requires {
			if requirement.ID == id {
				dependents = append(dependents, other)
				break
			}
		}
	}
	sort.Strings(dependents)
	return dependents
}

// missingRequirements lists the hard dependencies that are not installed or
// whose installed version is outside the requested range.
func (m *Manager) missingRequirements(manifest *protocol.Manifest) []protocol.ManifestRequirement {
	missing := make([]protocol.ManifestRequirement, 0)
	if manifest == nil {
		return missing
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, requirement := range manifest.Requires {
		item, ok := m.entries[requirement.ID]
		if !ok || item.manifest == nil {
			missing = append(missing, requirement)
			continue
		}
		if !VersionSatisfies(item.manifest.Version, requirement.Version) {
			missing = append(missing, requirement)
		}
	}
	return missing
}

// checkHostVersion enforces min_nginx_ui_version. A development build reports
// an empty version and is never rejected.
func checkHostVersion(manifest *protocol.Manifest) error {
	required := manifest.MinNginxUIVersion
	if required == "" {
		return nil
	}
	current := version.GetVersionInfo().Version
	if current == "" {
		return nil
	}
	if CompareVersions(current, required) < 0 {
		return cosy.WrapErrorWithParams(ErrHostVersionTooOld, required, current)
	}
	return nil
}

// normalizeSettings validates submitted values against the schema and keeps
// the stored secret whenever the redaction placeholder comes back unchanged.
func normalizeSettings(schema *protocol.SettingsSchema, values, stored map[string]any) (map[string]any, error) {
	if schema == nil {
		// Without a schema the plugin owns the shape of its settings.
		next := make(map[string]any, len(values))
		for key, value := range values {
			next[key] = value
		}
		return next, nil
	}

	next := make(map[string]any, len(schema.Settings))
	for _, field := range schema.Settings {
		raw, present := values[field.Key]
		if !present {
			if existing, ok := stored[field.Key]; ok {
				next[field.Key] = existing
			}
			continue
		}

		value, err := coerceSettingValue(field, raw, stored[field.Key])
		if err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		next[field.Key] = value
	}

	for _, field := range schema.Settings {
		if !field.Required {
			continue
		}
		value, ok := next[field.Key]
		if !ok || value == nil || value == "" {
			return nil, cosy.WrapErrorWithParams(ErrSettingsInvalid, field.Key)
		}
	}
	return next, nil
}

// coerceSettingValue converts one submitted value to the schema type.
func coerceSettingValue(field protocol.SettingsField, raw, stored any) (any, error) {
	invalid := func() (any, error) { return nil, cosy.WrapErrorWithParams(ErrSettingsInvalid, field.Key) }

	switch field.Type {
	case "bool":
		switch typed := raw.(type) {
		case bool:
			return typed, nil
		default:
			return invalid()
		}
	case "number":
		switch typed := raw.(type) {
		case float64, float32, int, int32, int64, uint, uint32, uint64:
			return cast.ToFloat64(typed), nil
		case json.Number:
			parsed, err := typed.Float64()
			if err != nil {
				return invalid()
			}
			return parsed, nil
		default:
			return invalid()
		}
	case "secret":
		text, ok := raw.(string)
		if !ok {
			return invalid()
		}
		if text == settings.RedactedSensitiveValue {
			// The form echoed the placeholder back, keep what is stored.
			return stored, nil
		}
		return text, nil
	case "select":
		text, ok := raw.(string)
		if !ok {
			return invalid()
		}
		if text == "" && !field.Required {
			return "", nil
		}
		if !slices.ContainsFunc(field.Options, func(option protocol.SettingsOption) bool {
			return option.Value == text
		}) {
			return invalid()
		}
		return text, nil
	default:
		text, ok := raw.(string)
		if !ok {
			return invalid()
		}
		return text, nil
	}
}
