package config

import (
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
)

func normalizeConfRelativePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimLeft(path, `/\`)

	if path == "" || path == "." {
		return ""
	}

	return path
}

// ResolveConfPath resolves a user-controlled path under the nginx config root.
// It rejects traversal attempts instead of clamping them back to the base directory.
func ResolveConfPath(parts ...string) (string, error) {
	confPath := filepath.Clean(nginx.GetConfPath())
	resolvedParts := []string{confPath}

	for _, part := range parts {
		normalized := normalizeConfRelativePath(part)
		if normalized == "" {
			continue
		}

		resolvedParts = append(resolvedParts, normalized)
	}

	resolvedPath := filepath.Clean(filepath.Join(resolvedParts...))
	if !helper.IsUnderDirectory(resolvedPath, confPath) {
		return "", cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, resolvedPath, confPath)
	}

	return resolvedPath, nil
}

// ResolveConfPathInDir resolves a user-controlled path under a fixed nginx config subdirectory.
func ResolveConfPathInDir(dir string, parts ...string) (string, error) {
	basePath, err := ResolveConfPath(dir)
	if err != nil {
		return "", err
	}

	resolvedParts := []string{basePath}
	for _, part := range parts {
		normalized := normalizeConfRelativePath(part)
		if normalized == "" {
			continue
		}

		resolvedParts = append(resolvedParts, normalized)
	}

	resolvedPath := filepath.Clean(filepath.Join(resolvedParts...))
	if !helper.IsUnderDirectory(resolvedPath, basePath) {
		return "", cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, resolvedPath, basePath)
	}

	return resolvedPath, nil
}

// ResolveConfPathInDirPreserveLeaf resolves a user-controlled path under a fixed nginx config
// subdirectory while preserving the final path component as-is.
// This is useful for paths whose leaf is expected to be a symlink, such as sites-enabled entries.
func ResolveConfPathInDirPreserveLeaf(dir string, parts ...string) (string, error) {
	basePath, err := ResolveConfPath(dir)
	if err != nil {
		return "", err
	}

	resolvedParts := []string{basePath}
	for _, part := range parts {
		normalized := normalizeConfRelativePath(part)
		if normalized == "" {
			continue
		}

		resolvedParts = append(resolvedParts, normalized)
	}

	resolvedPath := filepath.Clean(filepath.Join(resolvedParts...))
	parentPath := filepath.Dir(resolvedPath)
	if resolvedPath == basePath {
		parentPath = basePath
	}

	if !helper.IsUnderDirectory(parentPath, basePath) {
		return "", cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, resolvedPath, basePath)
	}

	return resolvedPath, nil
}

// ResolveAbsoluteOrRelativeConfPath validates an absolute path or resolves a relative one.
func ResolveAbsoluteOrRelativeConfPath(path string) (string, error) {
	confPath := filepath.Clean(nginx.GetConfPath())

	if filepath.IsAbs(path) {
		resolvedPath := filepath.Clean(path)
		if !helper.IsUnderDirectory(resolvedPath, confPath) {
			return "", cosy.WrapErrorWithParams(ErrPathIsNotUnderTheNginxConfDir, resolvedPath, confPath)
		}

		return resolvedPath, nil
	}

	return ResolveConfPath(path)
}
