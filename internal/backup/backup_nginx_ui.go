package backup

import (
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cosysettings "github.com/uozi-tech/cosy/settings"
)

// backupNginxUIFiles backs up the nginx-ui configuration and database files
func backupNginxUIFiles(destDir string) error {
	// Get config file path
	configPath := cosysettings.ConfPath
	if configPath == "" {
		return ErrConfigPathEmpty
	}

	// Always save the config file as app.ini, regardless of its original name
	destConfigPath := filepath.Join(destDir, "app.ini")
	if err := copyFile(configPath, destConfigPath); err != nil {
		return cosy.WrapErrorWithParams(ErrCopyConfigFile, err.Error())
	}

	// Snapshot through SQLite rather than copying the live file: committed WAL
	// pages belong in the backup, while reusable login sessions do not.
	dbPath := applicationDatabasePath()
	if _, err := os.Stat(dbPath); err == nil {
		destDBPath := filepath.Join(destDir, filepath.Base(dbPath))
		if err := snapshotBackupDatabase(dbPath, destDBPath); err != nil {
			return cosy.WrapErrorWithParams(ErrCopyDBFile, err.Error())
		}
	} else if os.IsNotExist(err) {
		logger.Warn("Database file not found: %s", dbPath)
	} else {
		return cosy.WrapErrorWithParams(ErrCopyDBFile, err.Error())
	}

	return nil
}

// backupNginxFiles backs up the nginx configuration directory
func backupNginxFiles(destDir string) error {
	// Get nginx config directory
	nginxConfigDir := nginx.GetConfPath()
	if nginxConfigDir == "" {
		return ErrNginxConfigDirEmpty
	}

	// Copy nginx config directory
	if err := copyBackupDirectory(nginxConfigDir, destDir); err != nil {
		return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, err.Error())
	}

	return nil
}
