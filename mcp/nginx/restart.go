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
	output, err := nginx.GetLastOutput()
	if err != nil {
		return mcp.NewToolResultError(output + "\n" + err.Error()), err
	}
	return mcp.NewToolResultText(output), nil
}
