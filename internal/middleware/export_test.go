package middleware

// ProxyServiceTokenRedactedRoutes reports the routes whose child node
// responses the node proxies rewrite or refuse for service tokens: those
// redacted for every service token, those redacted or refused for read-only
// tokens, and the WebSocket feeds refused to read-only tokens.
func ProxyServiceTokenRedactedRoutes() (allTokens, readOnly, refusedFeeds map[string]bool) {
	allTokens, readOnly, refusedFeeds = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for route := range serviceTokenRedactedResponseFields {
		allTokens[route] = true
	}
	for route := range readOnlyServiceTokenProxyRedactions {
		readOnly[route] = true
	}
	for route := range readOnlyServiceTokenRefusedProxyWebSockets {
		refusedFeeds[route] = true
	}
	return allTokens, readOnly, refusedFeeds
}
