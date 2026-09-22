package plugin

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// Package limits. They bound what a single archive can do to the host.
const (
	// MaxPackageFiles is the number of entries one package may contain.
	MaxPackageFiles = 10000
	// MaxPackageSize is the uncompressed size budget of one package.
	MaxPackageSize = 256 << 20
)

// invalidPackage wraps ErrPackageInvalid with the concrete reason.
func invalidPackage(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPackageInvalid, fmt.Sprintf(format, args...))
}

// ExtractPackage unpacks a .tar.gz plugin package into destDir and returns the
// validated manifest. Links, absolute paths and traversal are refused, and
// destDir is removed again when anything goes wrong.
func ExtractPackage(archivePath, destDir string) (manifest *protocol.Manifest, err error) {
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destDir)
		}
	}()

	prefix, err := packagePrefix(archivePath)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(destDir)
	if err != nil {
		return nil, err
	}

	err = walkPackage(archivePath, func(header *tar.Header, name string, reader io.Reader) error {
		rel, inside := strings.CutPrefix(name, prefix)
		if !inside || rel == "" {
			return nil
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !isInside(root, target) {
			return invalidPackage("entry %q escapes the destination", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			return os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Packed modes are ignored, markExecutables sets the bit on the
			// files the manifest runs.
			return writePackageFile(target, header, reader, 0o644)
		default:
			return invalidPackage("entry %q has an unsupported type", header.Name)
		}
	})
	if err != nil {
		return nil, err
	}

	manifest, err = LoadManifest(root)
	if err != nil {
		return nil, err
	}
	if err = ValidateManifest(manifest); err != nil {
		return nil, err
	}
	if err = markExecutables(manifest, root); err != nil {
		return nil, err
	}
	return manifest, nil
}

// BuildPackage writes srcDir into a .tar.gz package. It is used by the tests
// and by the plugin CLI.
func BuildPackage(srcDir, archivePath string) error {
	root, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			header := &tar.Header{Name: name + "/", Typeflag: tar.TypeDir, Mode: 0o755}
			return tw.WriteHeader(header)
		case info.Mode().IsRegular():
			header := &tar.Header{
				Name:     name,
				Typeflag: tar.TypeReg,
				Mode:     int64(info.Mode().Perm()),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
			}
			if err = tw.WriteHeader(header); err != nil {
				return err
			}
			file, err := os.Open(current)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tw, file)
			return err
		default:
			// Links and devices never make it into a package.
			return nil
		}
	})
	if err != nil {
		return err
	}
	if err = tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// packagePrefix scans the archive once to find where plugin.json lives and to
// enforce the entry limits before anything is written to disk.
func packagePrefix(archivePath string) (string, error) {
	var scan prefixScan
	err := walkPackage(archivePath, func(_ *tar.Header, name string, _ io.Reader) error {
		scan.add(name)
		return nil
	})
	if err != nil {
		return "", err
	}
	return scan.prefix()
}

// prefixScan collects what packagePrefix needs from the entry names.
type prefixScan struct {
	rootManifest bool
	topLevel     []string
	candidates   []string
}

// add records one entry name.
func (s *prefixScan) add(name string) {
	top, _, _ := strings.Cut(name, "/")
	if top != "" && !slices.Contains(s.topLevel, top) {
		s.topLevel = append(s.topLevel, top)
	}
	switch name {
	case ManifestFileName:
		s.rootManifest = true
	default:
		if path.Base(name) == ManifestFileName && strings.Count(name, "/") == 1 {
			s.candidates = append(s.candidates, path.Dir(name)+"/")
		}
	}
}

// prefix is the directory that holds plugin.json, empty for the archive root.
func (s *prefixScan) prefix() (string, error) {
	if s.rootManifest {
		return "", nil
	}
	// A single top level directory is stripped, which is what archives created
	// by "tar czf pkg.tgz plugin-dir" look like.
	if len(s.topLevel) == 1 && slices.Contains(s.candidates, s.topLevel[0]+"/") {
		return s.topLevel[0] + "/", nil
	}
	return "", invalidPackage("%s is missing at the package root", ManifestFileName)
}

// walkPackage streams the archive and calls visit for every safe entry. It
// enforces the file count, the size budget and the name rules.
func walkPackage(archivePath string, visit func(header *tar.Header, name string, reader io.Reader) error) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return invalidPackage("read gzip stream: %v", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var (
		files int
		total int64
	)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return invalidPackage("read tar stream: %v", err)
		}

		files++
		if files > MaxPackageFiles {
			return fmt.Errorf("%w: more than %d entries", ErrPackageTooLarge, MaxPackageFiles)
		}
		switch header.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			return invalidPackage("entry %q is a link", header.Name)
		case tar.TypeDir, tar.TypeReg:
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			continue
		default:
			return invalidPackage("entry %q has an unsupported type", header.Name)
		}

		name, err := packageEntryName(header.Name)
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg {
			if header.Size < 0 {
				return invalidPackage("entry %q has a negative size", header.Name)
			}
			total += header.Size
			if total > MaxPackageSize {
				return fmt.Errorf("%w: more than %d uncompressed bytes", ErrPackageTooLarge, int64(MaxPackageSize))
			}
		}
		if err = visit(header, name, tr); err != nil {
			return err
		}
	}
}

// packageEntryName normalises a tar entry name and refuses anything that could
// write outside the destination.
func packageEntryName(raw string) (string, error) {
	name := strings.TrimPrefix(filepath.ToSlash(raw), "./")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "." {
		return "", invalidPackage("entry %q has an empty name", raw)
	}
	if !isSafeRelPath(name) {
		return "", invalidPackage("entry %q is not a safe relative path", raw)
	}
	return name, nil
}

// writePackageFile writes one regular entry with mode.
func writePackageFile(target string, header *tar.Header, reader io.Reader, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.CopyN(file, reader, header.Size)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if written != header.Size {
		return invalidPackage("entry %q is truncated", header.Name)
	}
	return nil
}

// packedMode is the mode an entry was packed with, either 0644 or 0755.
func packedMode(header *tar.Header) os.FileMode {
	if header.FileInfo().Mode().Perm()&0o111 != 0 {
		return 0o755
	}
	return 0o644
}

// manifestExecutables lists the files the manifest runs: every
// server.executables value and server.command[0] when it is a path.
func manifestExecutables(m *protocol.Manifest) []string {
	if m.Server == nil {
		return nil
	}
	paths := make([]string, 0, len(m.Server.Executables)+1)
	for _, rel := range m.Server.Executables {
		paths = append(paths, rel)
	}
	if len(m.Server.Command) > 0 && hasPathSeparator(m.Server.Command[0]) {
		paths = append(paths, m.Server.Command[0])
	}
	return paths
}

// markExecutables makes every binary the manifest points at runnable. It is
// the only place an extracted file gets the executable bit.
func markExecutables(m *protocol.Manifest, root string) error {
	for _, rel := range manifestExecutables(m) {
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !isInside(root, target) {
			return invalidPackage("executable %q escapes the plugin directory", rel)
		}
		info, err := os.Stat(target)
		if os.IsNotExist(err) {
			// Other platforms' binaries are not shipped in every package.
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return invalidPackage("executable %q is not a regular file", rel)
		}
		if err = os.Chmod(target, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// isInside reports whether target stays under root.
func isInside(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
