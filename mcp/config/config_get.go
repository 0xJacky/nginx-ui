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
	relativePath := request.Params.Arguments["relative_path"].(string)

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
	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(absPath)).FirstOrCreate()
	if err != nil {
		return nil, err
	}

	cfg, err := q.Where(q.Filepath.Eq(absPath)).FirstOrInit()
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"name":              stat.Name(),
		"content":           string(content),
		"chat_gpt_messages": chatgpt.Content,
		"file_path":         absPath,
		"modified_at":       stat.ModTime(),
		"dir":               filepath.Dir(relativePath),
		"sync_node_ids":     cfg.SyncNodeIds,
		"sync_overwrite":    cfg.SyncOverwrite,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
