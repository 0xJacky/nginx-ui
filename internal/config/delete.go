package config

import (
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
)

// CleanupDatabaseRecords removes related database records after deletion
func CleanupDatabaseRecords(fullPath string, isDir bool) error {
	q := query.Config
	g := query.ChatGPTLog
	b := query.ConfigBackup

	if isDir {
		// For directories, clean up all records under the directory
		pathPattern := fullPath + "%"

		// Delete ChatGPT logs
		_, err := g.Where(g.Name.Like(pathPattern)).Delete()
		if err != nil {
			return err
		}

		// Delete config records
		_, err = q.Where(q.Filepath.Like(pathPattern)).Delete()
		if err != nil {
			return err
		}

		// Delete backup records
		_, err = b.Where(b.FilePath.Like(pathPattern)).Delete()
		if err != nil {
			return err
		}
	} else {
		// For files, delete specific records
		_, err := g.Where(g.Name.Eq(fullPath)).Delete()
		if err != nil {
			return err
		}

		_, err = q.Where(q.Filepath.Eq(fullPath)).Delete()
		if err != nil {
			return err
		}

		_, err = b.Where(b.FilePath.Eq(fullPath)).Delete()
		if err != nil {
			return err
		}
	}

	return nil
}

// IsProtectedPath checks if the path is protected and should not be deleted
func IsProtectedPath(fullPath, name string) bool {
	// Get nginx main config file path
	nginxConfPath := nginx.GetConfEntryPath()
	if fullPath == nginxConfPath {
		return true
	}

	// Protected directory names
	protectedDirs := []string{
		"sites-enabled",
		"sites-available",
		"streams-enabled",
		"streams-available",
		"conf.d",
	}

	for _, protected := range protectedDirs {
		if name == protected || strings.HasSuffix(fullPath, "/"+protected) {
			return true
		}
	}

	return false
}

// ValidateDeletePath validates that the path is safe to delete
func ValidateDeletePath(fullPath string) error {
	nginxConfPath := nginx.GetConfPath()
	if !IsUnderNginxConfDir(fullPath, nginxConfPath) {
		return ErrDeletePathNotUnderNginxConfDir
	}
	return nil
}

// IsUnderNginxConfDir checks if the given path is under nginx config directory
func IsUnderNginxConfDir(path, nginxConfPath string) bool {
	// Normalize paths
	path = strings.TrimSuffix(path, "/")
	nginxConfPath = strings.TrimSuffix(nginxConfPath, "/")

	// Check if path starts with nginx config path
	return strings.HasPrefix(path, nginxConfPath)
}

// CheckFileExists checks if file or directory exists and returns file info
func CheckFileExists(fullPath string) (os.FileInfo, error) {
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return stat, nil
}
