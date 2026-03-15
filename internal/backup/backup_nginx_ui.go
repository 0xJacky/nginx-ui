package backup

import (
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/settings"
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

	// Get database file name and path
	dbName := settings.DatabaseSettings.GetName()
	dbFile := dbName + ".db"

	// Database directory is the same as config file directory
	dbDir := filepath.Dir(configPath)
	dbPath := filepath.Join(dbDir, dbFile)

	// Copy database file
	if _, err := os.Stat(dbPath); err == nil {
		// Database exists as file
		destDBPath := filepath.Join(destDir, dbFile)
		if err := copyFile(dbPath, destDBPath); err != nil {
			return cosy.WrapErrorWithParams(ErrCopyDBFile, err.Error())
		}
	} else {
		logger.Warn("Database file not found: %s", dbPath)
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
	if err := copyDirectory(nginxConfigDir, destDir); err != nil {
		return cosy.WrapErrorWithParams(ErrCopyNginxConfigDir, err.Error())
	}

	return nil
}
