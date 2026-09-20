package mcp

import "github.com/gin-gonic/gin"

// IsServiceTokenRequest reports whether the current request was authenticated
// with a service token. Handlers can use this to remove credentials from
// otherwise readable resources without changing interactive administrator
// responses.
func IsServiceTokenRequest(c *gin.Context) bool {
	value, ok := c.Get(ServiceTokenPrincipalKey)
	if !ok {
		return false
	}

	_, ok = value.(*ServiceTokenPrincipal)
	return ok
}
