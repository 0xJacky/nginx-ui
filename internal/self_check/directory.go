package self_check

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// CheckSitesDirectory checks if sites-available/sites-enabled directory exists
func CheckSitesDirectory() error {
	// check sites-available directory
	if _, err := os.Stat(nginx.GetConfPath("sites-available")); os.IsNotExist(err) {
		return ErrSitesAvailableNotExist
	}

	// check sites-enabled directory
	if _, err := os.Stat(nginx.GetConfPath("sites-enabled")); os.IsNotExist(err) {
		return ErrSitesEnabledNotExist
	}

	return nil
}

// CheckStreamDirectory checks if stream-available/stream-enabled directory exists
func CheckStreamDirectory() error {
	// check stream-available directory
	if _, err := os.Stat(nginx.GetConfPath("streams-available")); os.IsNotExist(err) {
		return ErrStreamAvailableNotExist
	}

	// check stream-enabled directory
	if _, err := os.Stat(nginx.GetConfPath("streams-enabled")); os.IsNotExist(err) {
		return ErrStreamEnabledNotExist
	}

	return nil
}

// FixSitesDirectory creates sites-available/sites-enabled directory
func FixSitesDirectory() error {
	// create sites-available directory
	if err := os.MkdirAll(nginx.GetConfPath("sites-available"), 0755); err != nil {
		return err
	}

	// create sites-enabled directory
	if err := os.MkdirAll(nginx.GetConfPath("sites-enabled"), 0755); err != nil {
		return err
	}

	return nil
}

// FixStreamDirectory creates stream-available/stream-enabled directory
func FixStreamDirectory() error {
	// create stream-available directory
	if err := os.MkdirAll(nginx.GetConfPath("streams-available"), 0755); err != nil {
		return err
	}

	// create stream-enabled directory
	if err := os.MkdirAll(nginx.GetConfPath("streams-enabled"), 0755); err != nil {
		return err
	}

	return nil
}
