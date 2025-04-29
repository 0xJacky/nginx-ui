package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigAddToolName = "nginx_config_add"

// ErrFileAlreadyExists is returned when trying to create a file that already exists
var ErrFileAlreadyExists = errors.New("file already exists")

var nginxConfigAddTool = mcp.NewTool(
	nginxConfigAddToolName,
	mcp.WithDescription("Add or create a new Nginx configuration file"),
	mcp.WithString("name", mcp.Description("The name of the configuration file to create")),
	mcp.WithString("content", mcp.Description("The content of the configuration file")),
	mcp.WithString("base_dir", mcp.Description("The base directory for the configuration")),
	mcp.WithBoolean("overwrite", mcp.Description("Whether to overwrite an existing file")),
	mcp.WithArray("sync_node_ids", mcp.Description("IDs of nodes to sync the configuration to")),
)

func handleNginxConfigAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	name := args["name"].(string)
	content := args["content"].(string)
	baseDir := args["base_dir"].(string)
	overwrite := args["overwrite"].(bool)

	// Convert sync_node_ids from []interface{} to []uint64
	syncNodeIdsInterface, ok := args["sync_node_ids"].([]interface{})
	syncNodeIds := make([]uint64, 0)
	if ok {
		for _, id := range syncNodeIdsInterface {
			if idFloat, ok := id.(float64); ok {
				syncNodeIds = append(syncNodeIds, uint64(idFloat))
			}
		}
	}

	dir := nginx.GetConfPath(baseDir)
	path := filepath.Join(dir, name)
	if !helper.IsUnderDirectory(path, nginx.GetConfPath()) {
		return nil, config.ErrPathIsNotUnderTheNginxConfDir
	}

	if !overwrite && helper.FileExists(path) {
		return nil, ErrFileAlreadyExists
	}

	// Check if the directory exists, if not, create it
	if !helper.FileExists(dir) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return nil, err
	}

	output, err := nginx.Reload()
	if err != nil {
		return nil, err
	}

	if nginx.GetLogLevel(output) >= nginx.Warn {
		return nil, config.ErrNginxReloadFailed
	}

	q := query.Config
	_, err = q.Where(q.Filepath.Eq(path)).Delete()
	if err != nil {
		return nil, err
	}

	cfg := &model.Config{
		Name:          name,
		Filepath:      path,
		SyncNodeIds:   syncNodeIds,
		SyncOverwrite: overwrite,
	}

	err = q.Create(cfg)
	if err != nil {
		return nil, err
	}

	err = config.SyncToRemoteServer(cfg)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"name":      name,
		"content":   content,
		"file_path": path,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
