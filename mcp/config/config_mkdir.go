package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigMkdirToolName = "nginx_config_mkdir"

var nginxConfigMkdirTool = mcpgo.NewTool(
	nginxConfigMkdirToolName,
	mcpgo.WithDescription("Create a new directory in the Nginx configuration path"),
	mcpgo.WithString("base_path", mcpgo.Description("The base path where to create the directory")),
	mcpgo.WithString("folder_name", mcpgo.Description("The name of the folder to create")),
)

func handleNginxConfigMkdir(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	basePath := mcp.GetString(args, "base_path")
	folderName := mcp.GetString(args, "folder_name")

	if folderName == "" {
		return nil, fmt.Errorf("argument 'folder_name' is required")
	}

	fullPath, err := config.ResolveConfPath(basePath, folderName)
	if err != nil {
		return nil, err
	}

	err = os.Mkdir(fullPath, 0755)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"message": "Directory created successfully",
		"path":    fullPath,
	}

	jsonResult, _ := json.Marshal(result)
	return mcpgo.NewToolResultText(string(jsonResult)), nil
}
