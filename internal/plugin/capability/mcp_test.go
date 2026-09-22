package capability

import (
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"sync"
	"testing"

	internalmcp "github.com/0xJacky/Nginx-UI/internal/mcp"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/mark3labs/mcp-go/mcp"
)

const mcpPluginID = "io.github.example.cdn"

// fakeRegistry records what the bridge publishes.
type fakeRegistry struct {
	mu      sync.Mutex
	tools   map[string]internalmcp.Tool
	adds    int
	deletes int
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{tools: map[string]internalmcp.Tool{}} }

func (r *fakeRegistry) AddTools(tools ...internalmcp.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adds++
	for _, tool := range tools {
		r.tools[tool.Tool.Name] = tool
	}
}

func (r *fakeRegistry) DeleteTools(names ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletes++
	for _, name := range names {
		delete(r.tools, name)
	}
}

func (r *fakeRegistry) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *fakeRegistry) tool(name string) (internalmcp.Tool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool, ok := r.tools[name]
	return tool, ok
}

func mcpTools() []plugin.MCPToolEntry {
	return []plugin.MCPToolEntry{
		{PluginID: mcpPluginID, Tool: protocol.MCPTool{
			Name:        "purge_cache",
			Description: "Purge cached paths.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"zone": map[string]any{"type": "string"}},
				"required":   []any{"zone"},
			},
		}},
		{PluginID: mcpPluginID, Tool: protocol.MCPTool{Name: "list_zones", Description: "List zones."}},
	}
}

func TestMCPBridgePublishesAndWithdraws(t *testing.T) {
	host := newCapabilityHost()
	host.setTools(mcpTools())
	registry := newFakeRegistry()
	bridge := NewMCPBridge(host, registry)

	bridge.Sync()
	want := []string{"io_github_example_cdn__list_zones", "io_github_example_cdn__purge_cache"}
	if got := registry.names(); !slices.Equal(got, want) {
		t.Fatalf("published = %v, want %v", got, want)
	}

	// The schema is published unchanged and an absent one is an empty object.
	tool, _ := registry.tool("io_github_example_cdn__purge_cache")
	var schema map[string]any
	if err := json.Unmarshal(tool.Tool.RawInputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if schema["type"] != "object" || schema["required"].([]any)[0] != "zone" {
		t.Fatalf("schema = %v", schema)
	}
	if tool.Tool.Description != "Purge cached paths." {
		t.Fatalf("description = %q", tool.Tool.Description)
	}
	listZones, _ := registry.tool("io_github_example_cdn__list_zones")
	if string(listZones.Tool.RawInputSchema) != `{"type":"object"}` {
		t.Fatalf("default schema = %s", listZones.Tool.RawInputSchema)
	}

	// Nothing changed, nothing is sent to the server.
	adds, deletes := registry.adds, registry.deletes
	bridge.Sync()
	if registry.adds != adds || registry.deletes != deletes {
		t.Fatal("an unchanged inventory touched the server")
	}

	// A changed description republishes that tool only.
	changed := mcpTools()
	changed[1].Tool.Description = "List every zone."
	host.setTools(changed)
	bridge.Sync()
	listZones, _ = registry.tool("io_github_example_cdn__list_zones")
	if listZones.Tool.Description != "List every zone." {
		t.Fatalf("description = %q", listZones.Tool.Description)
	}

	// Disabling the plugin withdraws every tool.
	host.setTools(nil)
	bridge.Sync()
	if got := registry.names(); len(got) != 0 {
		t.Fatalf("published after disable = %v", got)
	}
	if len(bridge.Published()) != 0 {
		t.Fatalf("bridge still tracks %v", bridge.Published())
	}
}

func TestMCPBridgeSkipsAnInvalidSchema(t *testing.T) {
	host := newCapabilityHost()
	tools := mcpTools()
	// x-mcp-header only applies to primitive properties, the server would panic.
	tools[0].Tool.InputSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"zone": map[string]any{"type": "object", "x-mcp-header": "Zone"},
		},
	}
	host.setTools(tools)
	registry := newFakeRegistry()
	NewMCPBridge(host, registry).Sync()

	if got := registry.names(); !slices.Equal(got, []string{"io_github_example_cdn__list_zones"}) {
		t.Fatalf("published = %v", got)
	}
}

func callTool(t *testing.T, tool internalmcp.Tool, args any) *mcp.CallToolResult {
	t.Helper()
	request := mcp.CallToolRequest{}
	request.Params.Name = tool.Tool.Name
	request.Params.Arguments = args
	result, err := tool.Handler(t.Context(), request)
	if err != nil {
		t.Fatalf("handler error = %v, want a tool result", err)
	}
	return result
}

func resultText(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if text, ok := result.Content[0].(mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

func TestMCPToolCallsReachThePlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodMCPCall] = protocol.MCPCallResult{Content: []protocol.MCPContent{
		{Type: protocol.MCPContentTypeText, Text: "purged"},
		{Type: "image", Text: "ignored"},
	}}
	host := newCapabilityHost()
	host.callers[mcpPluginID] = caller
	host.setTools(mcpTools())
	registry := newFakeRegistry()
	NewMCPBridge(host, registry).Sync()

	tool, _ := registry.tool("io_github_example_cdn__purge_cache")
	result := callTool(t, tool, map[string]any{"zone": "example.com"})
	if result.IsError || len(result.Content) != 1 || resultText(result) != "purged" {
		t.Fatalf("result = %+v", result)
	}

	calls := caller.methodCalls(protocol.MethodMCPCall)
	if len(calls) != 1 {
		t.Fatalf("mcp.call calls = %d", len(calls))
	}
	var params protocol.MCPCallParams
	if err := json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Tool != "purge_cache" || params.Arguments["zone"] != "example.com" {
		t.Fatalf("params = %+v", params)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d", host.releaseCount())
	}

	// A tool error of the plugin stays a tool error.
	caller.results[protocol.MethodMCPCall] = protocol.MCPCallResult{
		Content: []protocol.MCPContent{{Type: protocol.MCPContentTypeText, Text: "zone not found"}},
		IsError: true,
	}
	if result = callTool(t, tool, map[string]any{"zone": "x"}); !result.IsError || resultText(result) != "zone not found" {
		t.Fatalf("result = %+v", result)
	}

	// A JSON-RPC error, bad arguments and an unavailable plugin become tool errors.
	caller.errs[protocol.MethodMCPCall] = &protocol.Error{Code: protocol.CodeInvalidParams, Message: "unknown tool: purge_cache"}
	if result = callTool(t, tool, nil); !result.IsError || resultText(result) == "" {
		t.Fatalf("result = %+v", result)
	}
	if result = callTool(t, tool, []any{"not", "an", "object"}); !result.IsError {
		t.Fatalf("result = %+v", result)
	}
	host.acquireErr = errors.New("plugin is not running")
	if result = callTool(t, tool, map[string]any{}); !result.IsError {
		t.Fatalf("result = %+v", result)
	}
}
