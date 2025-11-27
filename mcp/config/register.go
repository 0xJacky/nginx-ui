package config

import (
	"github.com/0xJacky/Nginx-UI/internal/mcp"
)

func Init() {
	mcp.AddTool(nginxConfigAddTool, handleNginxConfigAdd)
	mcp.AddTool(nginxConfigBasePathTool, handleNginxConfigBasePath)
	mcp.AddTool(nginxConfigEnableTool, handleNginxConfigEnable)
	mcp.AddTool(nginxConfigGetTool, handleNginxConfigGet)
	mcp.AddTool(nginxConfigHistoryTool, handleNginxConfigHistory)
	mcp.AddTool(nginxConfigListTool, handleNginxConfigList)
	mcp.AddTool(nginxConfigMkdirTool, handleNginxConfigMkdir)
	mcp.AddTool(nginxConfigModifyTool, handleNginxConfigModify)
	mcp.AddTool(nginxConfigRenameTool, handleNginxConfigRename)
}
