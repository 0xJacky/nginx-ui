package config

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigListToolName = "nginx_config_list"

var nginxConfigListTool = mcp.NewTool(
	nginxConfigListToolName,
	mcp.WithDescription("This is the list of Nginx configurations"),
	mcp.WithString("relative_path", mcp.Description("The relative path to the Nginx configurations")),
	mcp.WithString("filter_by_name", mcp.Description("Filter the Nginx configurations by name")),
)

func handleNginxConfigList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	relativePath := args["relative_path"].(string)
	filterByName := args["filter_by_name"].(string)
	configs, err := config.GetConfigList(relativePath, func(file os.FileInfo) bool {
		return filterByName == "" || strings.Contains(file.Name(), filterByName)
	})

	jsonResult, _ := json.Marshal(configs)

	return mcp.NewToolResultText(string(jsonResult)), err
}
