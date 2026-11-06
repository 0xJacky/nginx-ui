package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/0xJacky/Nginx-UI/settings"
)

// bundledStateFile lists, inside the plugin directory, the bundled plugins
// this node was already given. A plugin the user uninstalls afterwards stays
// uninstalled instead of coming back on the next boot.
const bundledStateFile = ".bundled.json"

type bundledState struct {
	Installed []string `json:"installed"`
}

// InstallBundledPackages installs the plugins the distribution ships in
// settings.PluginSettings.BundledDir: once per node when they are missing, and
// as an upgrade whenever the bundled package is newer than the installed one.
// Only packages signed with an official plugin key are installed this way.
func (m *Manager) InstallBundledPackages(ctx context.Context) error {
	if !settings.PluginSettings.Enabled || settings.PluginSettings.BundledDir == "" {
		return nil
	}

	packages, err := m.packagesIn(settings.PluginSettings.BundledDir)
	if err != nil {
		return err
	}

	state := m.readBundledState()
	recorded := len(state.Installed)
	for _, id := range packageIDs(packages) {
		pkg := newestPackage(packages, id)
		_, err := m.Get(id)
		installed := err == nil
		given := slices.Contains(state.Installed, id)
		if !installed && given {
			continue
		}

		// A first install is switched on with its permissions approved, an
		// upgrade keeps whatever the operator decided.
		opts := InstallOptions{Enable: !installed, ApprovePermissions: !installed, MinTrust: TrustOfficial}
		applied, err := m.applyLocalPackage(ctx, *pkg, opts)
		if err != nil {
			m.log.Warnf("[plugin:%s] bundled package: %v", id, err)
			continue
		}
		if applied {
			m.log.Infof("[plugin:%s] installed bundled %s", id, pkg.Manifest.Version)
		}
		if !given {
			state.Installed = append(state.Installed, id)
		}
	}

	if len(state.Installed) != recorded {
		return m.writeBundledState(state)
	}
	return nil
}

// installBundledPackage installs the newest bundled package of id regardless
// of the record of earlier installs. The DNS-01 auto install uses it when a
// certificate needs the plugin.
func (m *Manager) installBundledPackage(ctx context.Context, id string, opts InstallOptions) (*Info, error) {
	if settings.PluginSettings.BundledDir == "" {
		return nil, ErrLocalPackageNotFound
	}
	packages, err := m.packagesIn(settings.PluginSettings.BundledDir)
	if err != nil {
		return nil, err
	}
	pkg := newestPackage(packages, id)
	if pkg == nil {
		return nil, ErrLocalPackageNotFound
	}
	return m.Install(ctx, pkg.Path, opts)
}

// packageIDs returns the distinct plugin ids of packages in order.
func packageIDs(packages []LocalPackage) []string {
	ids := make([]string, 0, len(packages))
	for _, pkg := range packages {
		if !slices.Contains(ids, pkg.Manifest.ID) {
			ids = append(ids, pkg.Manifest.ID)
		}
	}
	return ids
}

func (m *Manager) readBundledState() bundledState {
	var state bundledState
	data, err := os.ReadFile(filepath.Join(m.Dir(), bundledStateFile))
	if err != nil {
		if !os.IsNotExist(err) {
			m.log.Warnf("Read %s: %v", bundledStateFile, err)
		}
		return state
	}
	if err = json.Unmarshal(data, &state); err != nil {
		m.log.Warnf("Parse %s: %v", bundledStateFile, err)
	}
	return state
}

func (m *Manager) writeBundledState(state bundledState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(m.Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.Dir(), bundledStateFile), data, 0o644)
}
