package nginx

import (
	"context"
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxStatusToolName = "nginx_status"

// statusResource is the status of the Nginx server
var statusTool = mcp.NewTool(
	nginxStatusToolName,
	mcp.WithDescription("This is the status of the Nginx server"),
)

// handleNginxStatus handles the Nginx status request
func handleNginxStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	lastOutput, err := nginx.GetLastOutput()
	if err != nil {
		return mcp.NewToolResultError(lastOutput + "\n" + err.Error()), err
	}

	running := nginx.IsNginxRunning()
	level := nginx.GetLogLevel(lastOutput)

	// build result
	result := gin.H{
		"running": running,
		"message": lastOutput,
		"level":   level,
	}

	// marshal to json and return text result
	jsonResult, _ := json.Marshal(result)

	return mcp.NewToolResultText(string(jsonResult)), nil
}
