package protocol

// MCPCallParams is the payload of mcp.call.
type MCPCallParams struct {
	// Tool is the tool name declared in the manifest, without the host prefix.
	Tool string `json:"tool"`
	// Arguments is the arguments object the MCP client sent.
	Arguments map[string]any `json:"arguments,omitempty"`
}

// MCPCallResult is the reply to mcp.call.
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	// IsError reports that the tool ran and failed. Content explains why.
	IsError bool `json:"is_error,omitempty"`
}

// MCPContent is one content block of a tool result.
type MCPContent struct {
	// Type is MCPContentTypeText, the only type the contract defines.
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPContentTypeText is the type of a text content block.
const MCPContentTypeText = "text"
