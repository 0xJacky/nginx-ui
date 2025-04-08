package config

import (
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
)

// CheckAndCreateHistory compares the provided content with the current content of the file
// at the specified path and creates a history record if they are different.
// The path must be under nginx.GetConfPath().
func CheckAndCreateHistory(path string, content string) error {
	// Check if path is under nginx.GetConfPath()
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		return ErrPathIsNotUnderTheNginxConfDir
	}

	// Read the current content of the file
	currentContent, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Compare the contents
	if string(currentContent) == content {
		// Contents are identical, no need to create history
		return nil
	}

	// Contents are different, create a history record (config backup)
	backup := &model.ConfigBackup{
		Name:     filepath.Base(path),
		FilePath: path,
		Content:  string(currentContent),
	}

	// Save the backup to the database
	cb := query.ConfigBackup
	err = cb.Create(backup)
	if err != nil {
		logger.Error("Failed to create config backup:", err)
		return err
	}

	return nil
}
