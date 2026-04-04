package config

import (
	"context"
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/query"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

const nginxConfigHistoryToolName = "nginx_config_history"

var nginxConfigHistoryTool = mcpgo.NewTool(
	nginxConfigHistoryToolName,
	mcpgo.WithDescription("Get history of Nginx configuration changes"),
	mcpgo.WithString("filepath", mcpgo.Description("The file path to get history for")),
)

func handleNginxConfigHistory(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := request.GetArguments()
	filepath := mcp.GetString(args, "filepath")

	q := query.ConfigBackup
	var histories, err = q.Where(q.FilePath.Eq(filepath)).Order(q.ID.Desc()).Find()
	if err != nil {
		return nil, err
	}

	jsonResult, _ := json.Marshal(histories)
	return mcpgo.NewToolResultText(string(jsonResult)), nil
}
