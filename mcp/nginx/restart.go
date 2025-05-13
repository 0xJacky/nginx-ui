package nginx

import (
	"context"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxRestartToolName = "restart_nginx"

var nginxRestartTool = mcp.NewTool(
	nginxRestartToolName,
	mcp.WithDescription("Perform a graceful restart of the Nginx configuration"),
)

func handleNginxRestart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	nginx.Restart()
	lastResult := nginx.GetLastResult()
	if lastResult.IsError() {
		return mcp.NewToolResultError(lastResult.GetOutput()), lastResult.GetError()
	}
	return mcp.NewToolResultText(lastResult.GetOutput()), nil
}
