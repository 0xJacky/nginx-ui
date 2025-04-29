package config

import (
	"context"
	"encoding/json"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigMkdirToolName = "nginx_config_mkdir"

var nginxConfigMkdirTool = mcp.NewTool(
	nginxConfigMkdirToolName,
	mcp.WithDescription("Create a new directory in the Nginx configuration path"),
	mcp.WithString("base_path", mcp.Description("The base path where to create the directory")),
	mcp.WithString("folder_name", mcp.Description("The name of the folder to create")),
)

func handleNginxConfigMkdir(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	basePath := args["base_path"].(string)
	folderName := args["folder_name"].(string)

	fullPath := nginx.GetConfPath(basePath, folderName)
	if !helper.IsUnderDirectory(fullPath, nginx.GetConfPath()) {
		return nil, config.ErrPathIsNotUnderTheNginxConfDir
	}

	err := os.Mkdir(fullPath, 0755)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"message": "Directory created successfully",
		"path":    fullPath,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
