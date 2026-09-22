package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/analytic"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

// Cluster sync pushes a package when a node cannot install from its own
// marketplace. With per-platform packages the package the controller
// installed only fits nodes of the controller platform, so a node on another
// platform gets a package resolved for its own platform instead:
//
//  1. the controller package, when it covers the node platform;
//  2. a package in the offline directory, packages/ or packages/installed/,
//     named in either form and holding the same id and version;
//  3. a package the controller fetched from the catalog for that platform
//     before, kept in packages/cache/;
//  4. the catalog download for that platform, fetched now into packages/cache/.
//
// When none exists the node is reported as unsupported_platform.

// platformCacheDirName holds the packages the controller fetched for the
// platforms of its nodes, relative to the offline package directory. The
// package scanner never looks into subdirectories.
const platformCacheDirName = "cache"

// errNoPlatformPackage marks a push that has nothing to send for the node
// platform, which the matrix reports as unsupported_platform.
var errNoPlatformPackage = errors.New("no package for the node platform")

// platformPackageMu serialises the lookup and the fetch of platform packages,
// so two nodes on the same platform never download the same file twice.
var platformPackageMu sync.Mutex

// ownArchive is the package the controller installed, shared by every node of
// one sync run.
type ownArchive struct {
	path string
	err  error
	// platforms is what the installed package runs on.
	platforms []string
}

// platform is the "<goos>-<goarch>" key of the node, empty when unknown.
func (n MatrixNode) platform() string {
	if n.OS == "" || n.Arch == "" {
		return ""
	}
	return n.OS + "-" + n.Arch
}

// statusPlatform reads the platform of one node from the monitor snapshot.
func statusPlatform(statuses analytic.TNodeMap, nodeID uint64) string {
	status, ok := statuses[nodeID]
	if !ok || status == nil || status.NodeRuntimeInfo.OS == "" || status.NodeRuntimeInfo.Arch == "" {
		return ""
	}
	return status.NodeRuntimeInfo.OS + "-" + status.NodeRuntimeInfo.Arch
}

// platformCacheDir is where the controller keeps the packages it fetched for
// other platforms.
func (m *Manager) platformCacheDir() string {
	return filepath.Join(m.PackagesDir(), platformCacheDirName)
}

// installedPlatforms lists the platforms the installed copy of a plugin runs
// on.
func (m *Manager) installedPlatforms(id string) []string {
	item, ok := m.lookup(id)
	if !ok {
		return nil
	}
	m.mu.RLock()
	dir, manifest := item.dir, item.manifest
	m.mu.RUnlock()
	if manifest == nil {
		return nil
	}
	return packagePlatforms(manifest, dir)
}

// pushArchive picks the package pushed to one node. A node of an unknown
// platform, or one the controller package covers, gets the controller package
// exactly as before.
func (m *Manager) pushArchive(ctx context.Context, info *Info, own *ownArchive, platform string) (string, error) {
	if platform == "" || platformsCover(own.platforms, platform) {
		return own.path, own.err
	}
	return m.platformPackage(ctx, info.ID, info.Version, platform)
}

// platformPackage resolves a package of one plugin version for a platform,
// looking at the local directories first and at the catalog last.
func (m *Manager) platformPackage(ctx context.Context, id, version, platform string) (string, error) {
	platformPackageMu.Lock()
	defer platformPackageMu.Unlock()

	if path, ok := m.localPlatformPackage(id, version, platform); ok {
		return path, nil
	}
	if !settings.PluginSettings.MarketplaceEnabled {
		return "", fmt.Errorf("%w %s: no local package and the marketplace is disabled", errNoPlatformPackage, platform)
	}
	return m.Marketplace().cachePlatformPackage(ctx, id, version, platform)
}

// localPlatformPackage looks for a usable package in the offline directory,
// the applied packages and the platform cache.
func (m *Manager) localPlatformPackage(id, version, platform string) (string, bool) {
	sources := []struct {
		dir string
		// trusted skips the signature check for files this node verified
		// itself when it fetched them.
		trusted bool
	}{
		{dir: m.PackagesDir()},
		{dir: filepath.Join(m.PackagesDir(), installedDirName)},
		{dir: m.platformCacheDir(), trusted: true},
	}
	names := []string{PackageFileName(id, version, platform), PackageFileName(id, version, "")}

	for _, source := range sources {
		for _, name := range names {
			path := filepath.Join(source.dir, name)
			if !fileExists(path) {
				continue
			}
			if err := m.checkPlatformPackage(path, id, version, platform, !source.trusted); err != nil {
				m.log.Warnf("[plugin:%s] skip %s for %s: %v", id, path, platform, err)
				continue
			}
			return path, true
		}
	}
	return "", false
}

// checkPlatformPackage makes sure a file on disk is the wanted plugin version
// and runs on platform, optionally verifying its detached signature.
func (m *Manager) checkPlatformPackage(path, id, version, platform string, verifySignature bool) error {
	if verifySignature {
		pkg := LocalPackage{Path: path}
		if signature := path + signatureSuffix; fileExists(signature) {
			pkg.SignaturePath = signature
		}
		if err := m.verifyLocalPackage(&pkg); err != nil {
			return err
		}
	}

	manifest, platforms, err := peekPackage(path)
	if err != nil {
		return err
	}
	if manifest.ID != id || manifest.Version != version {
		return fmt.Errorf("it holds %s %s", manifest.ID, manifest.Version)
	}
	if !platformsCover(platforms, platform) {
		return fmt.Errorf("it has no build for %s", platform)
	}
	return nil
}

// hasLocalPlatformPackage is the cheap check the matrix uses: a file named for
// the platform exists, whatever its content turns out to be.
func (m *Manager) hasLocalPlatformPackage(id, version, platform string) bool {
	name := PackageFileName(id, version, platform)
	for _, dir := range []string{m.PackagesDir(), filepath.Join(m.PackagesDir(), installedDirName), m.platformCacheDir()} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// pruneCachedPackages drops the cached packages of every other version of a
// plugin, only the version the controller runs is ever pushed.
func (m *Manager) pruneCachedPackages(id, keepVersion string) {
	dir := m.platformCacheDir()
	items, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, item := range items {
		name := strings.TrimSuffix(item.Name(), signatureSuffix)
		parsed, ok := ParsePackageFileName(name)
		if item.IsDir() || !ok || parsed.ID != id || parsed.Version == keepVersion {
			continue
		}
		_ = os.Remove(filepath.Join(dir, item.Name()))
	}
}

// cachePlatformPackage fetches the catalog download of one plugin version for
// a platform into the platform cache.
func (mp *Marketplace) cachePlatformPackage(ctx context.Context, id, version, platform string) (string, error) {
	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return "", err
	}
	entry := findEntry(entries, id, "")
	if entry == nil {
		return "", fmt.Errorf("%w %s: no local package and %s is not in the marketplace catalog",
			errNoPlatformPackage, platform, id)
	}
	release := findRelease(entry, version)
	if release == nil {
		return "", fmt.Errorf("%w %s: no local package and the catalog has no release %s",
			errNoPlatformPackage, platform, version)
	}
	if _, _, ok := release.DownloadFor(platform); !ok {
		return "", fmt.Errorf("%w %s: no local package and release %s ships %s",
			errNoPlatformPackage, platform, version, strings.Join(release.AvailablePlatforms(), ", "))
	}

	archive, err := mp.fetchRelease(ctx, entry, release, platform, mp.manager.platformCacheDir())
	if err != nil {
		return "", err
	}
	// The catalog promised this version for this platform, hold it to that.
	if err = mp.manager.checkPlatformPackage(archive, id, version, platform, false); err != nil {
		_ = os.Remove(archive)
		_ = os.Remove(archive + signatureSuffix)
		return "", cosy.WrapErrorWithParams(ErrCatalogInvalid, err.Error())
	}
	mp.manager.pruneCachedPackages(id, version)
	return archive, nil
}

// matrixCatalog loads the catalog for the matrix without letting a slow
// source hold the page up. Nil means no catalog information.
func (m *Manager) matrixCatalog(ctx context.Context) []CatalogEntry {
	if !settings.PluginSettings.Enabled || !settings.PluginSettings.MarketplaceEnabled {
		return nil
	}
	catalogCtx, cancel := context.WithTimeout(ctx, matrixTimeout)
	defer cancel()
	entries, err := m.Marketplace().Catalog(catalogCtx, false)
	if err != nil {
		return nil
	}
	return entries
}

// lazyCatalog loads the catalog the first time a matrix cell needs it, so a
// cluster whose nodes all share the controller platform never touches it.
type lazyCatalog struct {
	once    sync.Once
	load    func() []CatalogEntry
	entries []CatalogEntry
}

func (l *lazyCatalog) get() []CatalogEntry {
	if l == nil {
		return nil
	}
	l.once.Do(func() { l.entries = l.load() })
	return l.entries
}

// platformAvailability answers, for one matrix row, whether a node platform
// can get a package of the installed version.
type platformAvailability struct {
	manager   *Manager
	id        string
	version   string
	installed []string
	catalog   *lazyCatalog
	reasons   map[string]string
}

func (m *Manager) newPlatformAvailability(info Info, manifest *protocol.Manifest, catalog *lazyCatalog) *platformAvailability {
	availability := &platformAvailability{
		manager: m,
		id:      info.ID,
		version: info.Version,
		catalog: catalog,
		reasons: map[string]string{},
	}
	if manifest != nil {
		availability.installed = m.installedPlatforms(info.ID)
	}
	return availability
}

// unsupported reports whether nothing can be installed on platform, with the
// reason shown in the matrix cell. An unknown platform is never unsupported.
func (a *platformAvailability) unsupported(platform string) (string, bool) {
	if a == nil || platform == "" || a.installed == nil || platformsCover(a.installed, platform) {
		return "", false
	}
	if reason, ok := a.reasons[platform]; ok {
		return reason, reason != ""
	}
	reason := a.check(platform)
	a.reasons[platform] = reason
	return reason, reason != ""
}

func (a *platformAvailability) check(platform string) string {
	if a.manager.hasLocalPlatformPackage(a.id, a.version, platform) {
		return ""
	}
	if entry := findEntry(a.catalog.get(), a.id, ""); entry != nil {
		if release := findRelease(entry, a.version); release != nil {
			if _, _, ok := release.DownloadFor(platform); ok {
				return ""
			}
		}
	}
	return "no package for " + platform
}
