package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gen/field"
)

const nginxConfigModifyToolName = "nginx_config_modify"

// ErrFileNotFound is returned when a file is not found
var ErrFileNotFound = errors.New("file not found")

var nginxConfigModifyTool = mcpgo.NewTool(
	nginxConfigModifyToolName,
	mcpgo.WithDescription("Modify an existing Nginx configuration file"),
	mcpgo.WithString("relative_path", mcpgo.Description("The relative path to the configuration file")),
	mcpgo.WithString("content", mcpgo.Description("The new content of the configuration file")),
	mcpgo.WithBoolean("sync_overwrite", mcpgo.Description("Whether to overwrite existing files when syncing")),
	mcpgo.WithArray("sync_node_ids", mcpgo.Description("IDs of nodes to sync the configuration to")),
)

func handleNginxConfigModify(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	relativePath := mcp.GetString(args, "relative_path")
	content := mcp.GetString(args, "content")
	syncOverwrite := mcp.GetBool(args, "sync_overwrite")

	if relativePath == "" {
		return nil, fmt.Errorf("argument 'relative_path' is required")
	}
	if _, exists := args["content"]; !exists || args["content"] == nil {
		return nil, fmt.Errorf("argument 'content' is required")
	}

	// Convert sync_node_ids from []interface{} to []uint64
	syncNodeIdsInterface := mcp.GetSlice(args, "sync_node_ids")
	syncNodeIds := make([]uint64, 0)
	for _, id := range syncNodeIdsInterface {
		if idFloat, ok := id.(float64); ok {
			syncNodeIds = append(syncNodeIds, uint64(idFloat))
		}
	}

	absPath, err := config.ResolveAbsoluteOrRelativeConfPath(relativePath)
	if err != nil {
		return nil, err
	}

	if !helper.FileExists(absPath) {
		return nil, ErrFileNotFound
	}

	err = config.ValidateConfigFile(absPath, content)
	if err != nil {
		return nil, err
	}

	q := query.Config
	cfg, err := q.Assign(field.Attrs(&model.Config{
		Filepath: absPath,
		Name:     filepath.Base(absPath),
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
	return mcpgo.NewToolResultText(string(jsonResult)), nil
}
