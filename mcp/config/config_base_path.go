package config

import (
	"context"
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigBasePathToolName = "nginx_config_base_path"

var nginxConfigBasePathTool = mcp.NewTool(
	nginxConfigBasePathToolName,
	mcp.WithDescription("Get the base path of Nginx configurations"),
)

func handleNginxConfigBasePath(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	basePath := nginx.GetConfPath()

	result := map[string]interface{}{
		"base_path": basePath,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
