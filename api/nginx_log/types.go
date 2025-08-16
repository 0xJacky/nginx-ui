package nginx_log

import "github.com/0xJacky/Nginx-UI/internal/translation"

const (
	// PageSize defines the size of log chunks returned by the API
	PageSize = 128 * 1024
)

// controlStruct represents the request parameters for getting log content
type controlStruct struct {
	Type string `json:"type"` // Type of log: "access" or "error"
	Path string `json:"path"` // Path to the log file
}

// nginxLogPageResp represents the response format for log content
type nginxLogPageResp struct {
	Content string                 `json:"content"`         // Log content
	Page    int64                  `json:"page"`            // Current page number
	Error   *translation.Container `json:"error,omitempty"` // Error message if any
}