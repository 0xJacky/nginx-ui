package upstream

import (
	"net/http"
	"sort"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

// UpstreamInfo represents an upstream with its configuration and health status
type UpstreamInfo struct {
	Name       string                     `json:"name"`
	Servers    []upstream.ProxyTarget     `json:"servers"`
	ConfigPath string                     `json:"config_path"`
	LastSeen   string                     `json:"last_seen"`
	Status     map[string]*upstream.Status `json:"status"`
	Enabled    bool                       `json:"enabled"`
}

// GetUpstreamList returns all upstreams with their configuration and health status
func GetUpstreamList(c *gin.Context) {
	service := upstream.GetUpstreamService()

	// Get all upstream definitions
	upstreams := service.GetAllUpstreamDefinitions()

	// Get availability map
	availabilityMap := service.GetAvailabilityMap()

	// Get all upstream configurations from database
	u := query.UpstreamConfig
	configs, err := u.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create a map for quick lookup of enabled status by upstream name
	configMap := make(map[string]bool)
	for _, config := range configs {
		configMap[config.Socket] = config.Enabled
	}

	// Build response
	result := make([]UpstreamInfo, 0, len(upstreams))
	for name, def := range upstreams {
		// Get enabled status from database, default to true if not found
		enabled := true
		if val, exists := configMap[name]; exists {
			enabled = val
		}

		// Get status for each server in this upstream
		serverStatus := make(map[string]*upstream.Status)
		for _, server := range def.Servers {
			key := formatSocketAddress(server.Host, server.Port)
			if status, exists := availabilityMap[key]; exists {
				serverStatus[key] = status
			}
		}

		info := UpstreamInfo{
			Name:       name,
			Servers:    def.Servers,
			ConfigPath: def.ConfigPath,
			LastSeen:   def.LastSeen.Format("2006-01-02 15:04:05"),
			Status:     serverStatus,
			Enabled:    enabled,
		}
		result = append(result, info)
	}

	// Sort by name for stable ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	c.JSON(http.StatusOK, gin.H{
		"data": result,
	})
}

// UpdateUpstreamConfigRequest represents the request body for updating upstream config
type UpdateUpstreamConfigRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateUpstreamConfig updates the enabled status of an upstream
func UpdateUpstreamConfig(c *gin.Context) {
	name := c.Param("name")

	var req UpdateUpstreamConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	u := query.UpstreamConfig

	// Check if config exists
	config, err := u.Where(u.Socket.Eq(name)).First()
	if err != nil {
		// Create new config if not found
		config = &model.UpstreamConfig{
			Socket:  name,
			Enabled: req.Enabled,
		}
		if err := u.Create(config); err != nil {
			logger.Error("Failed to create upstream config:", err)
			cosy.ErrHandler(c, err)
			return
		}
	} else {
		// Update existing config
		if _, err := u.Where(u.Socket.Eq(name)).Update(u.Enabled, req.Enabled); err != nil {
			logger.Error("Failed to update upstream config:", err)
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Upstream config updated successfully",
	})
}

