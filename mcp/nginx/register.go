package nginx

import (
	"github.com/0xJacky/Nginx-UI/internal/mcp"
)

func Init() {
	mcp.AddTool(nginxReloadTool, handleNginxReload)
	mcp.AddTool(nginxRestartTool, handleNginxRestart)
	mcp.AddTool(statusTool, handleNginxStatus)
}
