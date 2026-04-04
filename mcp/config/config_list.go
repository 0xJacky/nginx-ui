package config

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigListToolName = "nginx_config_list"

var nginxConfigListTool = mcpgo.NewTool(
	nginxConfigListToolName,
	mcpgo.WithDescription("This is the list of Nginx configurations"),
	mcpgo.WithString("relative_path", mcpgo.Description("The relative path to the Nginx configurations")),
	mcpgo.WithString("filter_by_name", mcpgo.Description("Filter the Nginx configurations by name")),
)

func handleNginxConfigList(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	relativePath := mcp.GetString(args, "relative_path")
	filterByName := mcp.GetString(args, "filter_by_name")
	configs, err := config.GetConfigList(relativePath, func(file os.FileInfo) bool {
		return filterByName == "" || strings.Contains(file.Name(), filterByName)
	})

	jsonResult, _ := json.Marshal(configs)

	return mcpgo.NewToolResultText(string(jsonResult)), err
}
