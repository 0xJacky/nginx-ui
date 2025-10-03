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

// SocketInfo represents a socket with its configuration and health status
type SocketInfo struct {
	Socket       string           `json:"socket"`        // host:port
	Host         string           `json:"host"`          // hostname/IP
	Port         string           `json:"port"`          // port number
	Type         string           `json:"type"`          // proxy_pass, grpc_pass, or upstream
	IsConsul     bool             `json:"is_consul"`     // whether this is a consul service
	UpstreamName string           `json:"upstream_name"` // which upstream this belongs to (if any)
	LastCheck    string           `json:"last_check"`    // last time health check was performed
	Status       *upstream.Status `json:"status"`        // health check status
	Enabled      bool             `json:"enabled"`       // whether health check is enabled
}

// GetSocketList returns all sockets with their configuration and health status
func GetSocketList(c *gin.Context) {
	service := upstream.GetUpstreamService()

	// Get all target infos
	targets := service.GetTargetInfos()

	// Get availability map
	availabilityMap := service.GetAvailabilityMap()

	// Get all socket configurations from database
	u := query.UpstreamConfig
	configs, err := u.Find()
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	// Create a map for quick lookup of enabled status
	configMap := make(map[string]bool)
	for _, config := range configs {
		configMap[config.Socket] = config.Enabled
	}

	// Build response
	result := make([]SocketInfo, 0, len(targets))
	for _, target := range targets {
		socketAddr := formatSocketAddress(target.Host, target.Port)

		// Get enabled status from database, default to true if not found
		enabled := true
		if val, exists := configMap[socketAddr]; exists {
			enabled = val
		}

		// Get health status
		var status *upstream.Status
		if s, exists := availabilityMap[socketAddr]; exists {
			status = s
		}

		// Find which upstream this belongs to
		upstreamName := findUpstreamForSocket(service, target.ProxyTarget)

		info := SocketInfo{
			Socket:       socketAddr,
			Host:         target.Host,
			Port:         target.Port,
			Type:         target.Type,
			IsConsul:     target.IsConsul,
			UpstreamName: upstreamName,
			LastCheck:    target.LastSeen.Format("2006-01-02 15:04:05"),
			Status:       status,
			Enabled:      enabled,
		}
		result = append(result, info)
	}

	// Sort by socket address for stable ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Socket < result[j].Socket
	})

	c.JSON(http.StatusOK, gin.H{
		"data": result,
	})
}

// UpdateSocketConfigRequest represents the request body for updating socket config
type UpdateSocketConfigRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateSocketConfig updates the enabled status of a socket
func UpdateSocketConfig(c *gin.Context) {
	socket := c.Param("socket")

	var req UpdateSocketConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	u := query.UpstreamConfig

	// Check if config exists
	config, err := u.Where(u.Socket.Eq(socket)).First()
	if err != nil {
		// Create new config if not found
		config = &model.UpstreamConfig{
			Socket:  socket,
			Enabled: req.Enabled,
		}
		if err := u.Create(config); err != nil {
			logger.Error("Failed to create socket config:", err)
			cosy.ErrHandler(c, err)
			return
		}
	} else {
		// Update existing config
		if _, err := u.Where(u.Socket.Eq(socket)).Update(u.Enabled, req.Enabled); err != nil {
			logger.Error("Failed to update socket config:", err)
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Socket config updated successfully",
	})
}

// findUpstreamForSocket finds which upstream a socket belongs to
func findUpstreamForSocket(service *upstream.Service, target upstream.ProxyTarget) string {
	socketAddr := formatSocketAddress(target.Host, target.Port)
	upstreams := service.GetAllUpstreamDefinitions()

	for name, upstream := range upstreams {
		for _, server := range upstream.Servers {
			serverAddr := formatSocketAddress(server.Host, server.Port)
			if serverAddr == socketAddr {
				return name
			}
		}
	}
	return ""
}

