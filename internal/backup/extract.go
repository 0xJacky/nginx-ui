package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const maxArchiveSymlinkTargetSize = 16 * 1024

type zipEntryKind uint8

const (
	zipEntryDirectory zipEntryKind = iota
	zipEntryRegular
	zipEntrySymlink
)

type zipExtractionPolicy struct {
	absoluteSymlinkRewriteRoot string
	allowedAbsoluteLinkRoots   []string
}

type preparedZipEntry struct {
	file       *zip.File
	name       string
	kind       zipEntryKind
	linkTarget string
}

func extractZipArchive(zipPath, destDir string) error {
	return extractZipArchiveWithPolicy(zipPath, destDir, zipExtractionPolicy{})
}

func extractNginxZipArchive(zipPath, destDir, configRoot, modulesRoot string) error {
	policy := zipExtractionPolicy{
		absoluteSymlinkRewriteRoot: configRoot,
	}
	if strings.TrimSpace(modulesRoot) != "" {
		policy.allowedAbsoluteLinkRoots = []string{modulesRoot}
	}
	return extractZipArchiveWithPolicy(zipPath, destDir, policy)
}

func extractZipArchiveWithPolicy(zipPath, destDir string, policy zipExtractionPolicy) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %q: %w", zipPath, err)
	}
	defer reader.Close()

	entries, err := prepareZipEntries(reader.File, policy)
	if err != nil {
		return err
	}

	if err := ensureEmptyDirectory(destDir); err != nil {
		return err
	}

	root, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("open extraction root %q: %w", destDir, err)
	}
	defer root.Close()

	for _, entry := range entries {
		if entry.kind != zipEntryDirectory {
			continue
		}
		if err := root.MkdirAll(filepath.FromSlash(entry.name), directoryMode(entry.file.Mode())); err != nil {
			return fmt.Errorf("create archive directory %q: %w", entry.name, err)
		}
	}

	for _, entry := range entries {
		if entry.kind != zipEntryRegular {
			continue
		}
		if err := extractRegularZipEntry(root, entry); err != nil {
			return err
		}
	}

	// Symlinks are deliberately created last. No subsequent archive entry can
	// therefore redirect a file or directory write through a link.
	for _, entry := range entries {
		if entry.kind != zipEntrySymlink {
			continue
		}
		name := filepath.FromSlash(entry.name)
		if parent := filepath.Dir(name); parent != "." {
			if err := root.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("create parent for archive symlink %q: %w", entry.name, err)
			}
		}
		if err := root.Symlink(entry.linkTarget, name); err != nil {
			return fmt.Errorf("create archive symlink %q: %w", entry.name, err)
		}
	}

	return validateExtractedEntries(root, entries)
}

func prepareZipEntries(files []*zip.File, policy zipExtractionPolicy) ([]preparedZipEntry, error) {
	entries := make([]preparedZipEntry, 0, len(files))
	byName := make(map[string]zipEntryKind, len(files))

	for _, file := range files {
		name, err := normalizeArchiveEntryName(file.Name)
		if err != nil {
			return nil, err
		}

		kind, err := classifyZipEntry(file)
		if err != nil {
			return nil, fmt.Errorf("archive entry %q: %w", file.Name, err)
		}
		if _, exists := byName[name]; exists {
			return nil, fmt.Errorf("duplicate archive entry %q", name)
		}
		byName[name] = kind

		entry := preparedZipEntry{file: file, name: name, kind: kind}
		if kind == zipEntrySymlink {
			target, err := readZipSymlinkTarget(file)
			if err != nil {
				return nil, err
			}
			entry.linkTarget, err = normalizeArchiveSymlinkTarget(name, target, policy)
			if err != nil {
				return nil, err
			}
		}
		entries = append(entries, entry)
	}

	for name, kind := range byName {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if parentKind, exists := byName[parent]; exists && parentKind != zipEntryDirectory {
				return nil, fmt.Errorf("archive entry %q has non-directory ancestor %q", name, parent)
			}
		}
		if kind == zipEntryDirectory {
			continue
		}
		prefix := name + "/"
		for other := range byName {
			if strings.HasPrefix(other, prefix) {
				return nil, fmt.Errorf("archive entry %q conflicts with child entry %q", name, other)
			}
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		leftDepth := strings.Count(entries[i].name, "/")
		rightDepth := strings.Count(entries[j].name, "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return entries[i].name < entries[j].name
	})
	return entries, nil
}

func normalizeArchiveEntryName(raw string) (string, error) {
	if raw == "" || strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("invalid empty or NUL-containing archive path")
	}
	if strings.Contains(raw, "\\") {
		return "", fmt.Errorf("archive path %q contains a platform-dependent separator", raw)
	}
	if path.IsAbs(raw) || strings.HasPrefix(raw, "//") || hasWindowsVolumePrefix(raw) {
		return "", fmt.Errorf("archive path %q is absolute", raw)
	}

	clean := path.Clean(strings.TrimSuffix(raw, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive path %q escapes the extraction root", raw)
	}
	return clean, nil
}

func classifyZipEntry(file *zip.File) (zipEntryKind, error) {
	mode := file.Mode()
	if mode&os.ModeSymlink != 0 {
		if strings.HasSuffix(file.Name, "/") {
			return 0, fmt.Errorf("symlink is marked as a directory")
		}
		return zipEntrySymlink, nil
	}
	if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
		return zipEntryDirectory, nil
	}
	if mode&os.ModeType != 0 && !mode.IsRegular() {
		return 0, fmt.Errorf("unsupported special file mode %s", mode.String())
	}
	return zipEntryRegular, nil
}

func readZipSymlinkTarget(file *zip.File) (string, error) {
	if file.UncompressedSize64 > maxArchiveSymlinkTargetSize {
		return "", fmt.Errorf("archive symlink %q target is too large", file.Name)
	}
	source, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open archive symlink %q: %w", file.Name, err)
	}
	defer source.Close()

	target, err := io.ReadAll(io.LimitReader(source, maxArchiveSymlinkTargetSize+1))
	if err != nil {
		return "", fmt.Errorf("read archive symlink %q: %w", file.Name, err)
	}
	if len(target) > maxArchiveSymlinkTargetSize {
		return "", fmt.Errorf("archive symlink %q target is too large", file.Name)
	}
	return string(target), nil
}

func normalizeArchiveSymlinkTarget(entryName, rawTarget string, policy zipExtractionPolicy) (string, error) {
	if rawTarget == "" || strings.ContainsRune(rawTarget, '\x00') {
		return "", fmt.Errorf("archive symlink %q has an invalid target", entryName)
	}
	if strings.Contains(rawTarget, "\\") || hasWindowsVolumePrefix(rawTarget) {
		return "", fmt.Errorf("archive symlink %q has a platform-dependent target %q", entryName, rawTarget)
	}

	if filepath.IsAbs(rawTarget) || path.IsAbs(rawTarget) {
		cleanTarget := filepath.Clean(rawTarget)
		if policy.absoluteSymlinkRewriteRoot != "" && isPathWithin(cleanTarget, policy.absoluteSymlinkRewriteRoot) {
			relativeToRoot, err := filepath.Rel(filepath.Clean(policy.absoluteSymlinkRewriteRoot), cleanTarget)
			if err != nil {
				return "", fmt.Errorf("resolve archive symlink %q target: %w", entryName, err)
			}
			return relativeArchiveSymlinkTarget(entryName, filepath.ToSlash(relativeToRoot))
		}
		for _, allowedRoot := range policy.allowedAbsoluteLinkRoots {
			if allowedRoot != "" && isPathWithin(cleanTarget, allowedRoot) {
				return cleanTarget, nil
			}
		}
		return "", fmt.Errorf("archive symlink %q points outside allowed roots: %q", entryName, rawTarget)
	}

	return normalizeRelativeArchiveSymlinkTarget(entryName, rawTarget)
}

func normalizeRelativeArchiveSymlinkTarget(entryName, rawTarget string) (string, error) {
	cleanTarget := path.Clean(rawTarget)
	resolved := path.Clean(path.Join(path.Dir(entryName), cleanTarget))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", fmt.Errorf("archive symlink %q escapes the extraction root: %q", entryName, rawTarget)
	}

	return relativeArchiveSymlinkTarget(entryName, resolved)
}

func relativeArchiveSymlinkTarget(entryName, targetWithinRoot string) (string, error) {
	relativeTarget, err := filepath.Rel(
		filepath.FromSlash(path.Dir(entryName)),
		filepath.FromSlash(targetWithinRoot),
	)
	if err != nil {
		return "", fmt.Errorf("normalize archive symlink %q target: %w", entryName, err)
	}
	if relativeTarget == "." {
		return "", fmt.Errorf("archive symlink %q creates a self-referential target", entryName)
	}
	return relativeTarget, nil
}

func hasWindowsVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func isPathWithin(candidate, root string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}

func ensureEmptyDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create extraction directory %q: %w", directory, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("read extraction directory %q: %w", directory, err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("extraction directory %q is not empty", directory)
	}
	return nil
}

func extractRegularZipEntry(root *os.Root, entry preparedZipEntry) error {
	name := filepath.FromSlash(entry.name)
	parent := filepath.Dir(name)
	if parent != "." {
		if err := root.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("create parent for archive entry %q: %w", entry.name, err)
		}
	}

	destination, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, regularFileMode(entry.file.Mode()))
	if err != nil {
		return fmt.Errorf("create archive entry %q: %w", entry.name, err)
	}

	source, err := entry.file.Open()
	if err != nil {
		_ = destination.Close()
		return fmt.Errorf("open archive entry %q: %w", entry.name, err)
	}

	_, copyErr := io.Copy(destination, source)
	closeSourceErr := source.Close()
	closeDestinationErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("copy archive entry %q: %w", entry.name, copyErr)
	}
	if closeSourceErr != nil {
		return fmt.Errorf("close archive entry %q: %w", entry.name, closeSourceErr)
	}
	if closeDestinationErr != nil {
		return fmt.Errorf("close extracted entry %q: %w", entry.name, closeDestinationErr)
	}
	return nil
}

func validateExtractedEntries(root *os.Root, entries []preparedZipEntry) error {
	for _, entry := range entries {
		info, err := root.Lstat(filepath.FromSlash(entry.name))
		if err != nil {
			return fmt.Errorf("validate extracted entry %q: %w", entry.name, err)
		}
		switch entry.kind {
		case zipEntryDirectory:
			if !info.IsDir() {
				return fmt.Errorf("extracted entry %q is not a directory", entry.name)
			}
		case zipEntryRegular:
			if !info.Mode().IsRegular() {
				return fmt.Errorf("extracted entry %q is not a regular file", entry.name)
			}
		case zipEntrySymlink:
			if info.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("extracted entry %q is not a symlink", entry.name)
			}
			target, err := root.Readlink(filepath.FromSlash(entry.name))
			if err != nil {
				return fmt.Errorf("read extracted symlink %q: %w", entry.name, err)
			}
			if target != entry.linkTarget {
				return fmt.Errorf("extracted symlink %q target changed", entry.name)
			}
		}
	}
	return nil
}

func directoryMode(mode os.FileMode) os.FileMode {
	if permission := mode.Perm(); permission != 0 {
		return permission
	}
	return 0o755
}

func regularFileMode(mode os.FileMode) os.FileMode {
	if permission := mode.Perm(); permission != 0 {
		return permission
	}
	return 0o644
}
