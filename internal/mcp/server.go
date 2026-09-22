package mcp

import (
	"context"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	mcpServer = server.NewMCPServer(
		"Nginx",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)
	sseServer = server.NewSSEServer(
		mcpServer,
		server.WithSSEEndpoint("/mcp"),
		server.WithMessageEndpoint("/mcp_message"),
	)
)

const (
	MimeTypeJSON = "application/json"
	MimeTypeText = "text/plain"
)

type Resource struct {
	Resource mcp.Resource
	Handler  func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
}

type Tool struct {
	Tool    mcp.Tool
	Handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

var (
	tools     = make([]Tool, 0)
	toolMutex sync.Mutex
)

func AddTool(tool mcp.Tool, handler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	toolMutex.Lock()
	defer toolMutex.Unlock()
	tools = append(tools, Tool{Tool: tool, Handler: handler})
}

func ServeHTTP(c *gin.Context) {
	sseServer.ServeHTTP(c.Writer, c.Request)
}

func Init(ctx context.Context) {
	for _, tool := range tools {
		mcpServer.AddTool(tool.Tool, tool.Handler)
	}
}

// AddServerTools publishes tools on the live server right away and tells the
// connected clients the tool list changed. Built-in tools are collected with
// AddTool and published by Init instead; this is for tools that come and go
// at runtime, such as the ones plugins provide.
func AddServerTools(list ...Tool) {
	if len(list) == 0 {
		return
	}
	serverTools := make([]server.ServerTool, 0, len(list))
	for _, tool := range list {
		serverTools = append(serverTools, server.ServerTool{Tool: tool.Tool, Handler: tool.Handler})
	}
	mcpServer.AddTools(serverTools...)
}

// DeleteServerTools withdraws tools published with AddServerTools.
func DeleteServerTools(names ...string) {
	if len(names) == 0 {
		return
	}
	mcpServer.DeleteTools(names...)
}
