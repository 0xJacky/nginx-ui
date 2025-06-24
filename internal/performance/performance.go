package performance

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy/logger"
)

type NginxPerformanceInfo struct {
	StubStatusData
	NginxProcessInfo
	NginxConfigInfo
}

type NginxPerformanceResponse struct {
	StubStatusEnabled bool                 `json:"stub_status_enabled"`
	Running           bool                 `json:"running"`
	Info              NginxPerformanceInfo `json:"info"`
	Error             string               `json:"error"`
}

func GetPerformanceData() NginxPerformanceResponse {
	// Check if Nginx is running
	running := nginx.IsRunning()
	if !running {
		return NginxPerformanceResponse{
			StubStatusEnabled: false,
			Running:           false,
			Info:              NginxPerformanceInfo{},
		}
	}

	// Get Nginx status information
	stubStatusEnabled, statusInfo, err := GetStubStatusData()
	if err != nil {
		logger.Warn("Failed to get Nginx status:", err)
		return NginxPerformanceResponse{
			StubStatusEnabled: false,
			Running:           running,
			Info:              NginxPerformanceInfo{},
			Error:             err.Error(),
		}
	}

	// Get Nginx process information
	processInfo, err := GetNginxProcessInfo()
	if err != nil {
		logger.Warn("Failed to get Nginx process info:", err)
		return NginxPerformanceResponse{
			StubStatusEnabled: stubStatusEnabled,
			Running:           running,
			Info:              NginxPerformanceInfo{},
			Error:             err.Error(),
		}
	}

	// Get Nginx config information
	configInfo, err := GetNginxWorkerConfigInfo()
	if err != nil {
		logger.Warn("Failed to get Nginx config info:", err)
		return NginxPerformanceResponse{
			StubStatusEnabled: stubStatusEnabled,
			Running:           running,
			Info:              NginxPerformanceInfo{},
			Error:             err.Error(),
		}
	}

	// Ensure ProcessMode field is correctly passed
	perfInfo := NginxPerformanceInfo{
		StubStatusData:   *statusInfo,
		NginxProcessInfo: *processInfo,
		NginxConfigInfo:  *configInfo,
	}

	return NginxPerformanceResponse{
		StubStatusEnabled: stubStatusEnabled,
		Running:           running,
		Info:              perfInfo,
	}
}
