package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigRenameToolName = "nginx_config_rename"

var nginxConfigRenameTool = mcpgo.NewTool(
	nginxConfigRenameToolName,
	mcpgo.WithDescription("Rename a file or directory in the Nginx configuration path"),
	mcpgo.WithString("base_path", mcpgo.Description("The base path where the file or directory is located")),
	mcpgo.WithString("orig_name", mcpgo.Description("The original name of the file or directory")),
	mcpgo.WithString("new_name", mcpgo.Description("The new name for the file or directory")),
	mcpgo.WithArray("sync_node_ids", mcpgo.Description("IDs of nodes to sync the rename operation to")),
)

func handleNginxConfigRename(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	basePath := mcp.GetString(args, "base_path")
	origName := mcp.GetString(args, "orig_name")
	newName := mcp.GetString(args, "new_name")

	if origName == "" {
		return nil, fmt.Errorf("argument 'orig_name' is required")
	}
	if newName == "" {
		return nil, fmt.Errorf("argument 'new_name' is required")
	}

	// Convert sync_node_ids from []interface{} to []uint64
	syncNodeIdsInterface := mcp.GetSlice(args, "sync_node_ids")
	syncNodeIds := make([]uint64, 0)
	for _, id := range syncNodeIdsInterface {
		if idFloat, ok := id.(float64); ok {
			syncNodeIds = append(syncNodeIds, uint64(idFloat))
		}
	}

	if origName == newName {
		result := map[string]interface{}{
			"message": "No changes needed, names are identical",
		}
		jsonResult, _ := json.Marshal(result)
		return mcpgo.NewToolResultText(string(jsonResult)), nil
	}

	origFullPath, err := config.ResolveConfPath(basePath, origName)
	if err != nil {
		return nil, err
	}

	newFullPath, err := config.ResolveConfPath(basePath, newName)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(origFullPath)
	if err != nil {
		return nil, err
	}

	if helper.FileExists(newFullPath) {
		return nil, ErrFileAlreadyExists
	}

	err = os.Rename(origFullPath, newFullPath)
	if err != nil {
		return nil, err
	}

	// update LLM records
	g := query.LLMSession
	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(origFullPath)).FirstOrInit()
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		_, _ = g.Where(g.Path.Eq(newFullPath)).Delete()
		_, _ = g.Where(g.Path.Eq(origFullPath)).Update(g.Path, newFullPath)
		// for file, the sync policy for this file is used
		syncNodeIds = cfg.SyncNodeIds
	} else {
		// is directory, update all records under the directory
		_, _ = g.Where(g.Path.Like(origFullPath+"%")).Update(g.Path, g.Path.Replace(origFullPath, newFullPath))
	}

	_, err = q.Where(q.Filepath.Eq(origFullPath)).Updates(&model.Config{
		Filepath: newFullPath,
		Name:     newName,
	})
	if err != nil {
		return nil, err
	}

	b := query.ConfigBackup
	_, _ = b.Where(b.FilePath.Eq(origFullPath)).Updates(map[string]interface{}{
		"filepath": newFullPath,
		"name":     newName,
	})

	if len(syncNodeIds) > 0 {
		err = config.SyncRenameOnRemoteServer(origFullPath, newFullPath, syncNodeIds)
		if err != nil {
			return nil, err
		}
	}

	result := map[string]interface{}{
		"path": strings.TrimLeft(filepath.Join(basePath, newName), "/"),
	}

	jsonResult, _ := json.Marshal(result)
	return mcpgo.NewToolResultText(string(jsonResult)), nil
}
