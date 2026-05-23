package cert

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// SweepStalePending converts any pending cert records to failure.
// Called at server startup: a process restart kills in-flight issuance,
// leaving the WebSocket client with no way to receive a terminal status.
// Any record still marked pending at boot is by definition orphaned.
func SweepStalePending() error {
	db := model.UseDB()
	if db == nil {
		return nil
	}

	result := db.Model(&model.Cert{}).
		Where("status = ?", model.CertStatusPending).
		Updates(map[string]any{
			"status":     model.CertStatusFailure,
			"last_error": "Server restarted during issuance",
		})
	if result.Error != nil {
		logger.Errorf("SweepStalePending: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Infof("SweepStalePending: converted %d pending cert(s) to failure", result.RowsAffected)
	}
	return nil
}
