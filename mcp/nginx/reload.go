package nginx

import (
	"context"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxReloadToolName = "reload_nginx"

var nginxReloadTool = mcp.NewTool(
	nginxReloadToolName,
	mcp.WithDescription("Perform a graceful reload of the Nginx configuration"),
)

func handleNginxReload(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output, err := nginx.Reload()
	if err != nil {
		return mcp.NewToolResultError(output + "\n" + err.Error()), err
	}

	return mcp.NewToolResultText(output), nil
}
