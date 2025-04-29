package mcp

import (
	"github.com/0xJacky/Nginx-UI/mcp/config"
	"github.com/0xJacky/Nginx-UI/mcp/nginx"
)

func init() {
	config.Init()
	nginx.Init()
}
