package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigAddToolName = "nginx_config_add"

// ErrFileAlreadyExists is returned when trying to create a file that already exists
var ErrFileAlreadyExists = errors.New("file already exists")

var nginxConfigAddTool = mcpgo.NewTool(
	nginxConfigAddToolName,
	mcpgo.WithDescription("Add or create a new Nginx configuration file"),
	mcpgo.WithString("name", mcpgo.Description("The name of the configuration file to create")),
	mcpgo.WithString("content", mcpgo.Description("The content of the configuration file")),
	mcpgo.WithString("base_dir", mcpgo.Description("The base directory for the configuration")),
	mcpgo.WithBoolean("overwrite", mcpgo.Description("Whether to overwrite an existing file")),
	mcpgo.WithArray("sync_node_ids", mcpgo.Description("IDs of nodes to sync the configuration to")),
)

func handleNginxConfigAdd(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	name := mcp.GetString(args, "name")
	content := mcp.GetString(args, "content")
	baseDir := mcp.GetString(args, "base_dir")
	overwrite := mcp.GetBool(args, "overwrite")

	if name == "" {
		return nil, fmt.Errorf("argument 'name' is required")
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

	dir, err := config.ResolveConfPath(baseDir)
	if err != nil {
		return nil, err
	}

	path, err := config.ResolveConfPath(baseDir, name)
	if err != nil {
		return nil, err
	}

	err = config.ValidateConfigFile(path, content)
	if err != nil {
		return nil, err
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

	// Hold the apply lock for the whole write -> test -> reload sequence so a
	// concurrent mutation cannot make this call fail on somebody else's file.
	release := config.LockApply()
	defer release()

	tx := &config.FileTransaction{}
	if err = tx.Write(path, []byte(content), 0644); err != nil {
		return nil, config.RollbackError(err, tx.Rollback)
	}

	// A file Nginx rejects must not survive on disk. The running instance keeps
	// its valid in-memory configuration, so an untested write only breaks the
	// next Nginx start. A newly created file is removed by the rollback.
	if err = tx.TestAndReload(); err != nil {
		return nil, err
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
	return mcpgo.NewToolResultText(string(jsonResult)), nil
}
