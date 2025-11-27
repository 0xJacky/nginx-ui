package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigEnableToolName = "nginx_config_enable"

var nginxConfigEnableTool = mcp.NewTool(
	nginxConfigEnableToolName,
	mcp.WithDescription("Enable a previously created Nginx configuration (creates symlink in sites-enabled)"),
	mcp.WithString("name", mcp.Description("The name of the configuration file to enable")),
	mcp.WithString("base_dir", mcp.Description("The source directory (default: sites-available)")),
	mcp.WithBoolean("overwrite", mcp.Description("Whether to overwrite an existing enabled configuration")),
)

func handleNginxConfigEnable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name := args["name"].(string)
	baseDir := args["base_dir"].(string)
	overwrite := args["overwrite"].(bool)

	// Default to sites-available if base_dir is not provided
	if baseDir == "" {
		baseDir = "sites-available"
	}

	// Resolve Source Path (e.g., /etc/nginx/sites-available/my-site)
	// This is the file that must already exist.
	srcDir := nginx.GetConfPath(baseDir)
	srcPath := filepath.Join(srcDir, name)

	// Validate Source Exists
	if _, err := os.Stat(srcPath); err != nil {
		return nil, fmt.Errorf("source configuration file not found at %s: %w", srcPath, err)
	}

	// Resolve Destination Path (e.g., /etc/nginx/sites-enabled/my-site)
	// This is where the symlink will be created.
	dstDir := nginx.GetConfPath("sites-enabled")
	dstPath := filepath.Join(dstDir, name)

	// Ensure destination directory exists
	if !helper.FileExists(dstDir) {
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create sites-enabled directory: %w", err)
		}
	}

	// Check if Destination Already Exists
	if helper.FileExists(dstPath) {
		if !overwrite {
			return nil, fmt.Errorf("configuration is already enabled (symlink exists at %s)", dstPath)
		}
		// Remove existing symlink/file if overwrite is true
		if err := os.Remove(dstPath); err != nil {
			return nil, fmt.Errorf("failed to remove existing configuration at %s: %w", dstPath, err)
		}
	}

	// Create Symlink
	// We link srcPath -> dstPath
	if err := os.Symlink(srcPath, dstPath); err != nil {
		return nil, fmt.Errorf("failed to create symlink: %w", err)
	}

	// Test Nginx Configuration
	// As per internal/site/enable.go, we must verify config before reloading
	res := nginx.Control(nginx.TestConfig)
	if res.IsError() {
		// Revert change (remove symlink) if test fails to prevent breaking Nginx
		os.Remove(dstPath)
		return nil, fmt.Errorf("nginx config test failed: %v", res.GetError())
	}

	// Reload Nginx
	res = nginx.Control(nginx.Reload)
	if res.IsError() {
		return nil, fmt.Errorf("nginx reload failed: %v", res.GetError())
	}

	// Construct Success Response
	result := map[string]string{
		"status":      "success",
		"message":     "Site enabled and Nginx reloaded successfully",
		"source":      srcPath,
		"destination": dstPath,
	}
	jsonResult, _ := json.Marshal(result)

	return mcp.NewToolResultText(string(jsonResult)), nil

}