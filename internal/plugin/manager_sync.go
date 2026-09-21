package plugin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/model"
)

// ArchivesDirName holds one package per installed plugin so the controller can
// push the exact bytes it installed to a child node.
const ArchivesDirName = ".archives"

// archiveSuffix is the extension every kept package carries.
const archiveSuffix = ".tar.gz"

// syncers keeps one engine per manager. The process wide manager is the only
// one that ever runs an engine, the tests build their own.
var (
	syncersMu sync.Mutex
	syncers   = map[*Manager]*Syncer{}
)

// Syncer returns the cluster sync engine of this manager. The engine is inert
// until StartSync brings its worker up.
func (m *Manager) Syncer() *Syncer {
	syncersMu.Lock()
	defer syncersMu.Unlock()
	if existing, ok := syncers[m]; ok {
		return existing
	}
	created := newSyncer(m)
	syncers[m] = created
	return created
}

// StartSync brings the plugin cluster sync engine up. It is a no-op when the
// plugin system is disabled or the manager runs offline.
func (m *Manager) StartSync(ctx context.Context) {
	m.mu.RLock()
	offline := m.offline
	m.mu.RUnlock()
	if offline {
		return
	}
	m.Syncer().Start(ctx)
}

// StopSync tears the cluster sync engine down.
func (m *Manager) StopSync() {
	m.Syncer().Stop()
}

// archivesDir is the directory holding the kept packages.
func (m *Manager) archivesDir() string {
	return filepath.Join(m.Dir(), ArchivesDirName)
}

// ArchivePath returns the package kept for a plugin. Plugins installed by
// copying a directory into place have none.
func (m *Manager) ArchivePath(id string) (string, bool) {
	if !IsValidID(id) {
		return "", false
	}
	entries, err := os.ReadDir(m.archivesDir())
	if err != nil {
		return "", false
	}
	prefix := id + "-"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, archiveSuffix) {
			continue
		}
		return filepath.Join(m.archivesDir(), name), true
	}
	return "", false
}

// EnsureArchive returns a package for a plugin, building one from the files on
// disk when no package was kept. The returned cleanup removes a built package
// and must always be called.
func (m *Manager) EnsureArchive(id string) (string, func(), error) {
	noop := func() {}
	if path, ok := m.ArchivePath(id); ok {
		return path, noop, nil
	}

	item, ok := m.lookup(id)
	if !ok {
		return "", noop, ErrPluginNotFound
	}
	m.mu.RLock()
	dir := item.dir
	missing := item.manifest == nil
	m.mu.RUnlock()
	if missing {
		return "", noop, ErrPluginNotFound
	}

	staging, err := os.MkdirTemp("", "nginx-ui-plugin-archive-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(staging) }

	archive := filepath.Join(staging, id+archiveSuffix)
	if err = BuildPackage(dir, archive); err != nil {
		cleanup()
		return "", noop, err
	}
	return archive, cleanup, nil
}

// keepArchive stores a copy of the installed package, dropping the packages of
// the versions it replaces. Failing to keep it only costs an archive push.
func (m *Manager) keepArchive(id, version, archivePath string) {
	if !IsValidID(id) {
		return
	}
	dir := m.archivesDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		m.log.Warnf("[plugin:%s] create archive directory: %v", id, err)
		return
	}

	target := filepath.Join(dir, id+"-"+sanitizeArchiveVersion(version)+archiveSuffix)
	if err := copyFile(archivePath, target); err != nil {
		m.log.Warnf("[plugin:%s] keep package: %v", id, err)
		return
	}

	// Only the package matching the installed version is ever pushed.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := id + "-"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, archiveSuffix) {
			continue
		}
		if filepath.Join(dir, name) == target {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// sanitizeArchiveVersion keeps the file name predictable for any version
// string a manifest may carry.
func sanitizeArchiveVersion(version string) string {
	if version == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			return r
		case r == '.', r == '_', r == '+':
			return r
		default:
			return '_'
		}
	}, version)
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// syncRow snapshots the database row of a plugin, including the settings with
// their secrets, which never travel through the redacting API.
func (m *Manager) syncRow(id string) (*model.Plugin, bool) {
	item, ok := m.lookup(id)
	if !ok {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if item.row == nil {
		return nil, false
	}
	snapshot := *item.row
	snapshot.Settings = make(map[string]any, len(item.row.Settings))
	for key, value := range item.row.Settings {
		snapshot.Settings[key] = value
	}
	snapshot.SyncNodeIDs = append([]uint64(nil), item.row.SyncNodeIDs...)
	return &snapshot, true
}

// setSyncPolicy persists the cluster sync intent of one plugin.
func (m *Manager) setSyncPolicy(ctx context.Context, id, policy string, nodeIDs []uint64, syncSettings bool) error {
	m.opMu.Lock()
	item, ok := m.lookup(id)
	if !ok {
		m.opMu.Unlock()
		return ErrPluginNotFound
	}

	m.mu.Lock()
	row := item.row
	if row == nil {
		m.mu.Unlock()
		m.opMu.Unlock()
		return ErrPluginNotFound
	}
	row.SyncPolicy = policy
	row.SyncNodeIDs = append([]uint64(nil), nodeIDs...)
	row.SyncSettings = syncSettings
	m.mu.Unlock()

	err := m.saveRow(ctx, row)
	m.opMu.Unlock()
	return err
}

// autoPlugins lists the installed plugins whose policy asks for automatic
// installation on the child nodes.
func (m *Manager) autoPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.entries))
	for id, item := range m.entries {
		if item.manifest == nil || item.row == nil {
			continue
		}
		if item.row.SyncPolicy == model.PluginSyncPolicyAuto {
			ids = append(ids, id)
		}
	}
	return ids
}
