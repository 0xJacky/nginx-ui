package plugin

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

const (
	// PackagesDirName is the drop directory for offline installs, relative to
	// the plugin directory.
	PackagesDirName = "packages"
	// installedDirName holds the archives that were already applied.
	installedDirName = "installed"
	// packageSuffix is the only archive extension the scanner picks up.
	packageSuffix = ".tar.gz"
)

// LocalPackage is one archive found in the offline package directory. Its
// signature is embedded, Install verifies it.
type LocalPackage struct {
	// Path is the archive on disk.
	Path     string
	Manifest *protocol.Manifest
	// Platforms lists what the package runs on, see packagePlatforms.
	Platforms []string
}

// PackagesDir is the directory an operator drops packages into for an offline
// install, <pluginsDir>/packages.
func (m *Manager) PackagesDir() string {
	return filepath.Join(m.Dir(), PackagesDirName)
}

// ScanLocalPackages installs every package in the offline directory that is
// newer than what is installed, then moves the archive into packages/installed
// so the next boot does not look at it again.
func (m *Manager) ScanLocalPackages(ctx context.Context) error {
	if !settings.PluginSettings.Enabled {
		return nil
	}

	packages, err := m.localPackages()
	if err != nil {
		return err
	}

	for _, pkg := range packages {
		applied, err := m.applyLocalPackage(ctx, pkg, InstallOptions{})
		if err != nil {
			m.log.Warnf("Install local package %s: %v", filepath.Base(pkg.Path), err)
			continue
		}
		if applied {
			m.log.Infof("Installed %s %s from the local package directory", pkg.Manifest.ID, pkg.Manifest.Version)
		}
		m.archiveLocalPackage(&pkg)
	}
	return nil
}

// InstallLocalPackage installs one plugin from the offline directory. It is
// how the DNS-01 auto install stays usable on an air gapped node.
func (m *Manager) InstallLocalPackage(ctx context.Context, id string, opts InstallOptions) (*Info, error) {
	packages, err := m.localPackages()
	if err != nil {
		return nil, err
	}

	var best *LocalPackage
	for i := range packages {
		if packages[i].Manifest.ID != id {
			continue
		}
		if best == nil || CompareVersions(packages[i].Manifest.Version, best.Manifest.Version) > 0 {
			best = &packages[i]
		}
	}
	if best == nil {
		return nil, ErrLocalPackageNotFound
	}

	info, err := m.Install(ctx, best.Path, opts)
	if err != nil {
		return nil, err
	}
	m.archiveLocalPackage(best)
	return info, nil
}

// localPackages lists the readable archives in the offline directory that run
// on this node, newest version of each plugin last so a later install wins.
// Packages built for another platform stay where they are: cluster sync pushes
// them to the nodes that run that platform.
func (m *Manager) localPackages() ([]LocalPackage, error) {
	dir := m.PackagesDir()
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	host := HostPlatform()
	packages := make([]LocalPackage, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if item.IsDir() || !strings.HasSuffix(name, packageSuffix) {
			continue
		}
		// The file name already tells a foreign per-platform package apart,
		// which saves reading it on every boot.
		if parsed, ok := ParsePackageFileName(name); ok && parsed.Platform != "" && parsed.Platform != host {
			continue
		}
		archive := filepath.Join(dir, name)
		manifest, platforms, err := peekPackage(archive)
		if err != nil {
			m.log.Warnf("Skip local package %s: %v", name, err)
			continue
		}
		if !platformsCover(platforms, host) {
			m.log.Warnf("Skip local package %s: it has no build for %s", name, host)
			continue
		}
		packages = append(packages, LocalPackage{Path: archive, Manifest: manifest, Platforms: platforms})
	}

	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Manifest.ID != packages[j].Manifest.ID {
			return packages[i].Manifest.ID < packages[j].Manifest.ID
		}
		return CompareVersions(packages[i].Manifest.Version, packages[j].Manifest.Version) < 0
	})
	return packages, nil
}

// applyLocalPackage installs one archive when it is new or newer than what is
// on the node. It reports whether anything was installed.
func (m *Manager) applyLocalPackage(ctx context.Context, pkg LocalPackage, opts InstallOptions) (bool, error) {
	if current, err := m.Get(pkg.Manifest.ID); err == nil && current.Status != StatusMissing {
		if CompareVersions(pkg.Manifest.Version, current.Version) <= 0 {
			return false, nil
		}
		// An upgrade keeps whatever the operator had switched on.
		opts.Enable = opts.Enable || current.Enabled
	}

	if _, err := m.Install(ctx, pkg.Path, opts); err != nil {
		return false, err
	}
	return true, nil
}

// archiveLocalPackage moves an archive out of the scan path.
func (m *Manager) archiveLocalPackage(pkg *LocalPackage) {
	target := filepath.Join(m.PackagesDir(), installedDirName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		m.log.Warnf("Create %s: %v", target, err)
		return
	}
	destination := filepath.Join(target, filepath.Base(pkg.Path))
	_ = os.Remove(destination)
	if err := os.Rename(pkg.Path, destination); err != nil {
		m.log.Warnf("Archive %s: %v", filepath.Base(pkg.Path), err)
	}
}

// FetchPackage downloads the package one catalog release ships for platform
// into destDir so it can be carried to an offline node. The signature is
// embedded, the node that installs it verifies it. An empty platform means
// this node. It returns the archive path, named after the package form the
// catalog served.
func (mp *Marketplace) FetchPackage(ctx context.Context, id, wantVersion, platform, destDir string) (string, error) {
	if !IsValidID(id) {
		return "", ErrPluginNotFound
	}
	if platform == "" {
		platform = HostPlatform()
	}
	if platform != anyPlatform && !IsValidPlatform(platform) {
		return "", cosy.WrapErrorWithParams(ErrPlatformPackageMissing, platform)
	}

	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return "", err
	}
	entry := findEntry(entries, id, "")
	if entry == nil {
		return "", ErrMarketplaceNotFound
	}
	release, err := mp.resolveRelease(entry, wantVersion, platform)
	if err != nil {
		return "", err
	}
	return mp.fetchRelease(ctx, entry, release, platform, destDir)
}

// FetchTargets resolves "every platform" for one release: the version it
// picked and one platform per distinct package, which is what
// "plugin fetch --platform all" downloads. A platform the portable package
// serves is listed once for all of them.
func (mp *Marketplace) FetchTargets(ctx context.Context, id, wantVersion string) (string, []string, error) {
	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return "", nil, err
	}
	entry := findEntry(entries, id, "")
	if entry == nil {
		return "", nil, ErrMarketplaceNotFound
	}
	release := findRelease(entry, wantVersion)
	if release == nil {
		return "", nil, ErrReleaseNotFound
	}

	seen := map[string]bool{}
	targets := make([]string, 0, len(release.Downloads)+1)
	for _, platform := range release.AvailablePlatforms() {
		_, key, ok := release.DownloadFor(platform)
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, platform)
	}
	return release.Version, targets, nil
}

// fetchRelease downloads the package a release resolves to on platform into
// destDir and checks its digest. Nothing is published on the event bus: the
// caller is not an install.
func (mp *Marketplace) fetchRelease(ctx context.Context, entry *CatalogEntry, release *CatalogRelease,
	platform, destDir string,
) (string, error) {
	download, key, ok := release.DownloadFor(platform)
	if !ok {
		return "", cosy.WrapErrorWithParams(ErrPlatformPackageMissing, platform)
	}
	if err := mp.checkPolicy(entry, download.URL); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	archive := filepath.Join(destDir, PackageFileName(entry.ID, release.Version, key))
	// The archive only takes its final name once it is verified, so a failed
	// download never leaves something the package scanner would pick up.
	partial := archive + ".partial"
	defer os.Remove(partial)

	if err := mp.download(ctx, proxiedURL(download.URL), partial, nil); err != nil {
		return "", err
	}
	if err := verifyDigest(partial, download.SHA256); err != nil {
		return "", err
	}
	if err := os.Rename(partial, archive); err != nil {
		return "", err
	}
	return archive, nil
}

// findRelease returns one version of an entry, or the newest release that is
// not yanked when wantVersion is empty.
func findRelease(entry *CatalogEntry, wantVersion string) *CatalogRelease {
	var best *CatalogRelease
	for i := range entry.Releases {
		release := &entry.Releases[i]
		if wantVersion != "" {
			if release.Version == wantVersion {
				return release
			}
			continue
		}
		if release.Yanked {
			continue
		}
		if best == nil || CompareVersions(release.Version, best.Version) > 0 {
			best = release
		}
	}
	return best
}

// peekPackageManifest reads the manifest of an archive without unpacking it
// into the plugin directory.
func peekPackageManifest(archivePath string) (*protocol.Manifest, error) {
	manifest, _, err := peekPackage(archivePath)
	return manifest, err
}

// maxManifestSize caps how much of one plugin.json is held in memory.
const maxManifestSize = 1 << 20

// peekPackage reads the manifest of an archive and the platforms it runs on
// in one pass over the archive, without writing anything to disk. A broken
// package fails the way ExtractPackage fails.
func peekPackage(archivePath string) (*protocol.Manifest, []string, error) {
	var (
		scan prefixScan
		// files holds the regular entries, dirs the directories including the
		// implied parents.
		files     = map[string]bool{}
		dirs      = map[string]bool{}
		manifests = map[string][]byte{}
	)
	err := walkPackage(archivePath, func(header *tar.Header, name string, reader io.Reader) error {
		scan.add(name)
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			dirs[parent] = true
		}
		if header.Typeflag == tar.TypeDir {
			dirs[name] = true
			return nil
		}
		files[name] = true
		if path.Base(name) != ManifestFileName || strings.Count(name, "/") > 1 {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxManifestSize+1))
		if err != nil {
			return invalidPackage("read entry %q: %v", header.Name, err)
		}
		if len(data) > maxManifestSize {
			return invalidManifest("%s is larger than %d bytes", ManifestFileName, maxManifestSize)
		}
		manifests[name] = data
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	prefix, err := scan.prefix()
	if err != nil {
		return nil, nil, err
	}

	data, ok := manifests[prefix+ManifestFileName]
	if !ok {
		return nil, nil, invalidManifest("read %s: not a regular file", ManifestFileName)
	}
	manifest := &protocol.Manifest{}
	if err = json.Unmarshal(data, manifest); err != nil {
		return nil, nil, invalidManifest("decode %s: %v", ManifestFileName, err)
	}
	if err = ValidateManifest(manifest); err != nil {
		return nil, nil, err
	}
	// markExecutables refuses the same package on extraction.
	for _, rel := range manifestExecutables(manifest) {
		if dirs[prefix+rel] {
			return nil, nil, invalidPackage("executable %q is not a regular file", rel)
		}
	}
	platforms := packagePlatformsWith(manifest, func(rel string) bool { return files[prefix+rel] })
	return manifest, platforms, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
