package plugin

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/pkgsign"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
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
	// signatureSuffix is the detached minisign signature next to an archive.
	signatureSuffix = ".minisig"
)

// LocalPackage is one archive found in the offline package directory.
type LocalPackage struct {
	// Path is the archive on disk.
	Path string
	// SignaturePath is the sibling .minisig, empty when there is none.
	SignaturePath string
	Manifest      *protocol.Manifest
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

	if err = m.verifyLocalPackage(best); err != nil {
		return nil, err
	}
	info, err := m.Install(ctx, best.Path, opts)
	if err != nil {
		return nil, err
	}
	m.archiveLocalPackage(best)
	return info, nil
}

// localPackages lists the readable archives in the offline directory, newest
// version of each plugin last so a later install wins.
func (m *Manager) localPackages() ([]LocalPackage, error) {
	dir := m.PackagesDir()
	items, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	packages := make([]LocalPackage, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if item.IsDir() || !strings.HasSuffix(name, packageSuffix) {
			continue
		}
		archive := filepath.Join(dir, name)
		manifest, err := peekPackageManifest(archive)
		if err != nil {
			m.log.Warnf("Skip local package %s: %v", name, err)
			continue
		}
		pkg := LocalPackage{Path: archive, Manifest: manifest}
		if signature := archive + signatureSuffix; fileExists(signature) {
			pkg.SignaturePath = signature
		}
		packages = append(packages, pkg)
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

	if err := m.verifyLocalPackage(&pkg); err != nil {
		return false, err
	}
	if _, err := m.Install(ctx, pkg.Path, opts); err != nil {
		return false, err
	}
	return true, nil
}

// verifyLocalPackage checks the detached signature when there is one, or when
// the node refuses unsigned packages. There is no digest to check here, the
// archive is the only artifact.
func (m *Manager) verifyLocalPackage(pkg *LocalPackage) error {
	if pkg.SignaturePath == "" {
		if settings.PluginSettings.RequireSignature {
			return ErrSignatureMissing
		}
		return nil
	}

	signature, err := os.ReadFile(pkg.SignaturePath)
	if err != nil {
		return err
	}
	keys := append(releasesign.TrustedPublicKeys(), settings.PluginSettings.TrustedPublicKeys...)
	if _, err = pkgsign.VerifyFile(pkg.Path, signature, keys); err != nil {
		return cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
	}
	return nil
}

// archiveLocalPackage moves an archive and its signature out of the scan path.
func (m *Manager) archiveLocalPackage(pkg *LocalPackage) {
	target := filepath.Join(m.PackagesDir(), installedDirName)
	if err := os.MkdirAll(target, 0o755); err != nil {
		m.log.Warnf("Create %s: %v", target, err)
		return
	}
	for _, source := range []string{pkg.Path, pkg.SignaturePath} {
		if source == "" {
			continue
		}
		destination := filepath.Join(target, filepath.Base(source))
		_ = os.Remove(destination)
		if err := os.Rename(source, destination); err != nil {
			m.log.Warnf("Archive %s: %v", filepath.Base(source), err)
		}
	}
}

// FetchPackage downloads one catalog release and its signature into destDir so
// it can be carried to an offline node. It returns the archive path.
func (mp *Marketplace) FetchPackage(ctx context.Context, id, wantVersion, destDir string) (string, error) {
	if !IsValidID(id) {
		return "", ErrPluginNotFound
	}

	entries, err := mp.Catalog(ctx, false)
	if err != nil {
		return "", err
	}
	entry := findEntry(entries, id, "")
	if entry == nil {
		return "", ErrMarketplaceNotFound
	}
	release, err := mp.resolveRelease(entry, wantVersion)
	if err != nil {
		return "", err
	}
	if err = mp.checkPolicy(entry, release); err != nil {
		return "", err
	}
	if err = os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	archive := filepath.Join(destDir, id+"-"+release.Version+packageSuffix)
	if err = mp.download(ctx, id, proxiedURL(release.DownloadURL), archive); err != nil {
		return "", err
	}
	if err = verifyDigest(archive, release.SHA256); err != nil {
		return "", err
	}

	signatureURL := release.SignatureURL
	if signatureURL == "" {
		signatureURL = release.DownloadURL + signatureSuffix
	}
	signature, err := fetchText(ctx, proxiedURL(signatureURL), maxReadmeBytes)
	switch {
	case err != nil || strings.TrimSpace(signature) == "":
		if isOfficialSource(entry.Source) || settings.PluginSettings.RequireSignature {
			return "", ErrSignatureMissing
		}
	default:
		if _, err = pkgsign.VerifyFile(archive, []byte(signature), mp.trustedKeys(entry, release)); err != nil {
			return "", cosy.WrapErrorWithParams(ErrSignatureInvalid, err.Error())
		}
		if err = os.WriteFile(archive+signatureSuffix, []byte(signature), 0o644); err != nil {
			return "", err
		}
	}
	return archive, nil
}

// peekPackageManifest reads the manifest of an archive without unpacking it
// into the plugin directory.
func peekPackageManifest(archivePath string) (*protocol.Manifest, error) {
	staging, err := os.MkdirTemp("", "nginx-ui-plugin-peek-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)

	return ExtractPackage(archivePath, filepath.Join(staging, "payload"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
