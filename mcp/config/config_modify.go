package config

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gen/field"
)

const nginxConfigModifyToolName = "nginx_config_modify"

// ErrFileNotFound is returned when a file is not found
var ErrFileNotFound = errors.New("file not found")

var nginxConfigModifyTool = mcp.NewTool(
	nginxConfigModifyToolName,
	mcp.WithDescription("Modify an existing Nginx configuration file"),
	mcp.WithString("relative_path", mcp.Description("The relative path to the configuration file")),
	mcp.WithString("content", mcp.Description("The new content of the configuration file")),
	mcp.WithBoolean("sync_overwrite", mcp.Description("Whether to overwrite existing files when syncing")),
	mcp.WithArray("sync_node_ids", mcp.Description("IDs of nodes to sync the configuration to")),
)

func handleNginxConfigModify(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	relativePath := args["relative_path"].(string)
	content := args["content"].(string)
	syncOverwrite := args["sync_overwrite"].(bool)

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

	absPath := nginx.GetConfPath(relativePath)
	if !helper.IsUnderDirectory(absPath, nginx.GetConfPath()) {
		return nil, config.ErrPathIsNotUnderTheNginxConfDir
	}

	if !helper.FileExists(absPath) {
		return nil, ErrFileNotFound
	}

	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Filepath: absPath,
	})).Where(q.Filepath.Eq(absPath)).FirstOrCreate()
	if err != nil {
		return nil, err
	}

	// Update database record
	_, err = q.Where(q.Filepath.Eq(absPath)).
		Select(q.SyncNodeIds, q.SyncOverwrite).
		Updates(&model.Config{
			SyncNodeIds:   syncNodeIds,
			SyncOverwrite: syncOverwrite,
		})
	if err != nil {
		return nil, err
	}

	cfg.SyncNodeIds = syncNodeIds
	cfg.SyncOverwrite = syncOverwrite

	err = config.Save(absPath, content, cfg)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"name":           filepath.Base(absPath),
		"content":        content,
		"file_path":      absPath,
		"dir":            filepath.Dir(relativePath),
		"sync_node_ids":  cfg.SyncNodeIds,
		"sync_overwrite": cfg.SyncOverwrite,
	}

	jsonResult, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
