package upstream

import (
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

func init() {
	// Register the disabled sockets checker callback
	service := upstream.GetUpstreamService()
	service.SetDisabledSocketsChecker(getDisabledSockets)
}

// getDisabledSockets queries the database for disabled sockets
func getDisabledSockets() map[string]bool {
	disabled := make(map[string]bool)

	db := model.UseDB()
	if db == nil {
		return disabled
	}

	var configs []model.UpstreamConfig
	if err := db.Where("enabled = ?", false).Find(&configs).Error; err != nil {
		logger.Error("Failed to query disabled sockets:", err)
		return disabled
	}

	for _, config := range configs {
		disabled[config.Socket] = true
	}

	return disabled
}
