package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigGetToolName = "nginx_config_get"

var nginxConfigGetTool = mcp.NewTool(
	nginxConfigGetToolName,
	mcp.WithDescription("Get a specific Nginx configuration file"),
	mcp.WithString("relative_path", mcp.Description("The relative path to the configuration file")),
)

func handleNginxConfigGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	relativePath := args["relative_path"].(string)

	absPath := nginx.GetConfPath(relativePath)
	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		return nil, config.ErrPathIsNotUnderTheNginxConfDir
	}

	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(absPath)).FirstOrInit()
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"name":           stat.Name(),
		"content":        string(content),
		"file_path":      absPath,
		"modified_at":    stat.ModTime(),
		"dir":            filepath.Dir(relativePath),
		"sync_node_ids":  cfg.SyncNodeIds,
		"sync_overwrite": cfg.SyncOverwrite,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
