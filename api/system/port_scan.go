package system

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
)

type PortScanRequest struct {
	StartPort int `json:"start_port" binding:"required,min=1,max=65535"`
	EndPort   int `json:"end_port" binding:"required,min=1,max=65535"`
	Page      int `json:"page" binding:"required,min=1"`
	PageSize  int `json:"page_size" binding:"required,min=1,max=1000"`
}

type PortInfo struct {
	Port    int    `json:"port"`
	Status  string `json:"status"`
	Process string `json:"process"`
}

type PortScanResponse struct {
	Data     []PortInfo `json:"data"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

func PortScan(c *gin.Context) {
	var req PortScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}

	if req.StartPort > req.EndPort {
		c.JSON(400, gin.H{"message": "Start port must be less than or equal to end port"})
		return
	}

	// Calculate pagination
	totalPorts := req.EndPort - req.StartPort + 1
	startIndex := (req.Page - 1) * req.PageSize
	endIndex := startIndex + req.PageSize

	if startIndex >= totalPorts {
		c.JSON(200, PortScanResponse{
			Data:     []PortInfo{},
			Total:    totalPorts,
			Page:     req.Page,
			PageSize: req.PageSize,
		})
		return
	}

	if endIndex > totalPorts {
		endIndex = totalPorts
	}

	// Calculate actual port range for this page
	actualStartPort := req.StartPort + startIndex
	actualEndPort := req.StartPort + endIndex - 1

	var ports []PortInfo

	// Get listening ports info
	listeningPorts := getListeningPorts()

	// Scan ports in the current page range
	for port := actualStartPort; port <= actualEndPort; port++ {
		portInfo := PortInfo{
			Port:    port,
			Status:  "closed",
			Process: "",
		}

		// Check if port is listening
		if processInfo, exists := listeningPorts[port]; exists {
			portInfo.Status = "listening"
			portInfo.Process = processInfo
		} else {
			// Quick check if port is open but not in listening list
			if isPortOpen(port) {
				portInfo.Status = "open"
			}
		}

		ports = append(ports, portInfo)
	}

	c.JSON(200, PortScanResponse{
		Data:     ports,
		Total:    totalPorts,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

func isPortOpen(port int) bool {
	timeout := time.Millisecond * 100
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func getListeningPorts() map[int]string {
	ports := make(map[int]string)

	// Try netstat first
	if cmd := exec.Command("netstat", "-tlnp"); cmd.Err == nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "LISTEN") {
					fields := strings.Fields(line)
					if len(fields) >= 4 {
						address := fields[3]
						process := ""
						if len(fields) >= 7 {
							process = fields[6]
						}

						// Extract port from address (format: 0.0.0.0:port or :::port)
						if colonIndex := strings.LastIndex(address, ":"); colonIndex != -1 {
							portStr := address[colonIndex+1:]
							if port, err := strconv.Atoi(portStr); err == nil {
								ports[port] = process
							}
						}
					}
				}
			}
			return ports
		}
	}

	// Fallback to ss command
	if cmd := exec.Command("ss", "-tlnp"); cmd.Err == nil {
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "LISTEN") {
					fields := strings.Fields(line)
					if len(fields) >= 4 {
						address := fields[3]
						process := ""
						if len(fields) >= 6 {
							process = fields[5]
						}

						// Extract port from address
						if colonIndex := strings.LastIndex(address, ":"); colonIndex != -1 {
							portStr := address[colonIndex+1:]
							if port, err := strconv.Atoi(portStr); err == nil {
								ports[port] = process
							}
						}
					}
				}
			}
		}
	}

	logger.Debug("Found listening ports: %v", ports)
	return ports
}
