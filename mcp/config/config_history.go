package config

import (
	"context"
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/query"
	"github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigHistoryToolName = "nginx_config_history"

var nginxConfigHistoryTool = mcp.NewTool(
	nginxConfigHistoryToolName,
	mcp.WithDescription("Get history of Nginx configuration changes"),
	mcp.WithString("filepath", mcp.Description("The file path to get history for")),
)

func handleNginxConfigHistory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filepath := request.Params.Arguments["filepath"].(string)

	q := query.ConfigBackup
	var histories, err = q.Where(q.FilePath.Eq(filepath)).Order(q.ID.Desc()).Find()
	if err != nil {
		return nil, err
	}

	jsonResult, _ := json.Marshal(histories)
	return mcp.NewToolResultText(string(jsonResult)), nil
}
