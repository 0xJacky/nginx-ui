package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy"
)

func CheckGeoLiteDB() error {
	// Only check if log indexing is enabled
	if !settings.NginxLogSettings.IndexingEnabled {
		return nil
	}

	availability := geolite.CurrentAvailability()
	if !availability.Exists {
		return cosy.WrapErrorWithParams(ErrGeoLiteDBNotFound, availability.Path)
	}

	return nil
}

func FixGeoLiteDB() error {
	// This is a placeholder function to mark the task as fixable
	// The actual fix is handled by the frontend modal
	return ErrTaskNotFixable
}
