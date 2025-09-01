package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigRenameToolName = "nginx_config_rename"

var nginxConfigRenameTool = mcp.NewTool(
	nginxConfigRenameToolName,
	mcp.WithDescription("Rename a file or directory in the Nginx configuration path"),
	mcp.WithString("base_path", mcp.Description("The base path where the file or directory is located")),
	mcp.WithString("orig_name", mcp.Description("The original name of the file or directory")),
	mcp.WithString("new_name", mcp.Description("The new name for the file or directory")),
	mcp.WithArray("sync_node_ids", mcp.Description("IDs of nodes to sync the rename operation to")),
)

func handleNginxConfigRename(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	basePath := args["base_path"].(string)
	origName := args["orig_name"].(string)
	newName := args["new_name"].(string)

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

	if origName == newName {
		result := map[string]interface{}{
			"message": "No changes needed, names are identical",
		}
		jsonResult, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(jsonResult)), nil
	}

	origFullPath := nginx.GetConfPath(basePath, origName)
	newFullPath := nginx.GetConfPath(basePath, newName)
	if !helper.IsUnderDirectory(origFullPath, nginx.GetConfPath()) ||
		!helper.IsUnderDirectory(newFullPath, nginx.GetConfPath()) {
		return nil, config.ErrPathIsNotUnderTheNginxConfDir
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
	g := query.LLMMessages
	q := query.Config
	cfg, err := q.Where(q.Filepath.Eq(origFullPath)).FirstOrInit()
	if err != nil {
		return nil, err
	}

	if !stat.IsDir() {
		_, _ = g.Where(g.Name.Eq(newFullPath)).Delete()
		_, _ = g.Where(g.Name.Eq(origFullPath)).Update(g.Name, newFullPath)
		// for file, the sync policy for this file is used
		syncNodeIds = cfg.SyncNodeIds
	} else {
		// is directory, update all records under the directory
		_, _ = g.Where(g.Name.Like(origFullPath+"%")).Update(g.Name, g.Name.Replace(origFullPath, newFullPath))
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
	return mcp.NewToolResultText(string(jsonResult)), nil
}
