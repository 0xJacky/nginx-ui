package capability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/event"
	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/uozi-tech/cosy/logger"
)

// mcpCallTimeout bounds one tool call, including starting an on_demand plugin.
const mcpCallTimeout = 60 * time.Second

// MCPHost is the part of the plugin manager the mcp capability needs.
type MCPHost interface {
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// MCPTools lists the tools of every enabled plugin that was granted the
	// mcp permission.
	MCPTools() []plugin.MCPToolEntry
}

// MCPToolRegistry is where plugin tools are published, the live MCP server
// outside of tests.
type MCPToolRegistry interface {
	AddTools(tools ...internalmcp.Tool)
	DeleteTools(names ...string)
}

// serverRegistry publishes on the nginx-ui MCP server.
type serverRegistry struct{}

func (serverRegistry) AddTools(tools ...internalmcp.Tool) { internalmcp.AddServerTools(tools...) }
func (serverRegistry) DeleteTools(names ...string)        { internalmcp.DeleteServerTools(names...) }

// RegisterMCP publishes the tools of the plugins of h on the MCP server and
// keeps them in step with the plugin inventory: a tool appears when its
// plugin is enabled with the mcp permission granted and disappears when the
// plugin is disabled, uninstalled or needs a new approval.
func RegisterMCP(h MCPHost) {
	bridge := NewMCPBridge(h, serverRegistry{})
	bridge.Sync()
	event.Subscribe(func(published event.Event) {
		if published.Type != event.TypePluginChanged {
			return
		}
		// Bus subscribers must not block.
		go bridge.Sync()
	})
}

// MCPBridge publishes plugin tools on a registry and forwards their calls.
type MCPBridge struct {
	host     MCPHost
	registry MCPToolRegistry

	mu        sync.Mutex
	published map[string]plugin.MCPToolEntry
}

// NewMCPBridge returns a bridge that has published nothing yet.
func NewMCPBridge(h MCPHost, registry MCPToolRegistry) *MCPBridge {
	return &MCPBridge{host: h, registry: registry, published: map[string]plugin.MCPToolEntry{}}
}

// Sync aligns the published tools with what the enabled plugins declare now.
// A tool whose declaration changed is withdrawn and published again.
func (b *MCPBridge) Sync() {
	b.mu.Lock()
	defer b.mu.Unlock()

	desired := map[string]plugin.MCPToolEntry{}
	for _, entry := range b.host.MCPTools() {
		desired[plugin.MCPToolName(entry.PluginID, entry.Tool.Name)] = entry
	}

	var stale []string
	for name, current := range b.published {
		if want, ok := desired[name]; !ok || !reflect.DeepEqual(want, current) {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		b.registry.DeleteTools(stale...)
		for _, name := range stale {
			delete(b.published, name)
		}
	}

	var added []internalmcp.Tool
	for name, entry := range desired {
		if _, ok := b.published[name]; ok {
			continue
		}
		tool, err := b.serverTool(name, entry)
		if err != nil {
			logger.Warnf("[plugin:%s] skip mcp tool %s: %v", entry.PluginID, entry.Tool.Name, err)
			continue
		}
		added = append(added, tool)
		b.published[name] = entry
	}
	if len(added) > 0 {
		b.registry.AddTools(added...)
	}
}

// Published lists the published tool names, for diagnostics and tests.
func (b *MCPBridge) Published() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	names := make([]string, 0, len(b.published))
	for name := range b.published {
		names = append(names, name)
	}
	return names
}

// serverTool builds the MCP tool a plugin tool is published as.
func (b *MCPBridge) serverTool(name string, entry plugin.MCPToolEntry) (internalmcp.Tool, error) {
	schema := entry.Tool.InputSchema
	if schema == nil {
		schema = map[string]any{"type": "object"}
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return internalmcp.Tool{}, fmt.Errorf("encode input_schema: %w", err)
	}

	tool := mcp.NewToolWithRawSchema(name, entry.Tool.Description, raw)
	// The server panics on a schema whose header annotations it rejects.
	if err = mcp.ValidateParamHeaderAnnotations(&tool); err != nil {
		return internalmcp.Tool{}, err
	}
	return internalmcp.Tool{Tool: tool, Handler: b.handler(entry.PluginID, entry.Tool.Name)}, nil
}

// handler forwards a tool call to mcp.call. Every failure becomes a tool
// error result, so the MCP client sees why instead of a protocol error.
func (b *MCPBridge) handler(pluginID, tool string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := request.GetArguments()
		if args == nil && request.Params.Arguments != nil {
			return mcp.NewToolResultError("the arguments must be a JSON object"), nil
		}

		callCtx, cancel := context.WithTimeout(ctx, mcpCallTimeout)
		defer cancel()

		caller, release, err := b.host.Acquire(callCtx, pluginID)
		if release != nil {
			defer release()
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("plugin %s is not available: %v", pluginID, err)), nil
		}

		var result protocol.MCPCallResult
		err = caller.Call(callCtx, protocol.MethodMCPCall, protocol.MCPCallParams{Tool: tool, Arguments: args}, &result)
		if err != nil {
			if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
				return mcp.NewToolResultError(fmt.Sprintf("plugin %s did not answer within %s", pluginID, mcpCallTimeout)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("plugin %s failed: %s", pluginID, rpcMessage(err))), nil
		}
		return toolResult(result), nil
	}
}

// toolResult converts the plugin reply. Content types this host does not
// know are skipped (spec MCP-6).
func toolResult(result protocol.MCPCallResult) *mcp.CallToolResult {
	content := make([]mcp.Content, 0, len(result.Content))
	for _, block := range result.Content {
		if block.Type != protocol.MCPContentTypeText {
			continue
		}
		content = append(content, mcp.NewTextContent(block.Text))
	}
	return &mcp.CallToolResult{Content: content, IsError: result.IsError}
}
