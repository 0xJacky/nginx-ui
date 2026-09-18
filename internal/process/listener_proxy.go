package process

import "github.com/pires/go-proxyproto"

// ConfigureProxyProtocol must be called once at startup, before accepting any
// connections. Risefront sends PROXY headers for TCP connections but forwards
// Unix connections without them. SKIP accepts the latter without trusting
// address headers supplied by a client. TCP keeps the dependency's default.
func ConfigureProxyProtocol(network string) {
	if network == "unix" {
		proxyproto.DefaultPolicy = proxyproto.SKIP
	}
}
