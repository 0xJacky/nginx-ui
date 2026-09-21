package protocol

// HostInfo describes the host to the plugin during the handshake.
type HostInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Locale  string `json:"locale"`
}

// InitializeParams is the payload of plugin.initialize.
type InitializeParams struct {
	Host        HostInfo       `json:"host"`
	Settings    map[string]any `json:"settings"`
	Permissions []string       `json:"permissions"`
}

// InitializeResult is the reply to plugin.initialize.
type InitializeResult struct {
	APIVersion   int      `json:"api_version"`
	Capabilities []string `json:"capabilities"`
	// Transports lists the transports the plugin can serve, e.g. ["stdio","grpc"].
	// Empty means stdio only.
	Transports []string `json:"transports,omitempty"`
	// HTTPPort is reported by Windows plugins that serve the http capability
	// on a loopback port instead of a Unix socket.
	HTTPPort int `json:"http_port,omitempty"`
	// RPCPort and RPCToken are reported by Windows plugins that serve gRPC on
	// a loopback port instead of a Unix socket.
	RPCPort  int    `json:"rpc_port,omitempty"`
	RPCToken string `json:"rpc_token,omitempty"`
}

// ConfigureParams is the payload of plugin.configure.
type ConfigureParams struct {
	Settings map[string]any `json:"settings"`
}

// EmptyResult is the reply to methods that have nothing to return.
type EmptyResult struct{}
