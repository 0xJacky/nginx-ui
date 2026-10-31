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
	// HTTPPipe is the named pipe a Windows plugin serves the http capability
	// on. HTTPPort is the loopback port of a Windows plugin that cannot open
	// a named pipe, used when HTTPPipe is empty.
	HTTPPipe string `json:"http_pipe,omitempty"`
	HTTPPort int    `json:"http_port,omitempty"`
	// RPCPipe is the named pipe a Windows plugin serves gRPC on. RPCPort is
	// the loopback port of a Windows plugin that cannot open a named pipe,
	// used when RPCPipe is empty. Both require RPCToken.
	RPCPipe  string `json:"rpc_pipe,omitempty"`
	RPCPort  int    `json:"rpc_port,omitempty"`
	RPCToken string `json:"rpc_token,omitempty"`
	// RPCSocket is the absolute path of the Unix socket the gRPC transport
	// listens on. Empty means <NGINX_UI_PLUGIN_DATA_DIR>/rpc.sock.
	RPCSocket string `json:"rpc_socket,omitempty"`
}

// ConfigureParams is the payload of plugin.configure.
type ConfigureParams struct {
	Settings map[string]any `json:"settings"`
}

// EmptyResult is the reply to methods that have nothing to return.
type EmptyResult struct{}
