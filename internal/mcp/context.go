package mcp

import (
	"context"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

// IsServiceTokenRequest reports whether the current request was authenticated
// with a service token. Handlers can use this to remove credentials from
// otherwise readable resources without changing interactive administrator
// responses.
func IsServiceTokenRequest(c *gin.Context) bool {
	_, ok := serviceTokenPrincipal(c)
	return ok
}

// IsReadOnlyServiceTokenRequest reports whether the current request was
// authenticated with a service token that lacks api:write. api:read is a
// read-only role: it keeps every non-secret field but must never receive a
// stored secret. Interactive administrators and api:write management tokens
// are unaffected.
func IsReadOnlyServiceTokenRequest(c *gin.Context) bool {
	principal, ok := serviceTokenPrincipal(c)
	return ok && !principal.HasScope(model.APITokenScopeWrite)
}

func serviceTokenPrincipal(c *gin.Context) (*ServiceTokenPrincipal, bool) {
	value, ok := c.Get(ServiceTokenPrincipalKey)
	if !ok {
		return nil, false
	}

	principal, ok := value.(*ServiceTokenPrincipal)
	return principal, ok && principal != nil
}

type readOnlyServiceTokenContextKey struct{}

// WithReadOnlyServiceToken marks a request context as serving a service token
// that lacks mcp:write. MCP tool handlers only receive the request context, not
// the Gin context that holds the principal, so the MCP router records the
// decision here before the request reaches the MCP server.
func WithReadOnlyServiceToken(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyServiceTokenContextKey{}, true)
}

// IsReadOnlyServiceTokenContext reports whether WithReadOnlyServiceToken
// marked ctx, in which case MCP tools must hide stored secrets.
func IsReadOnlyServiceTokenContext(ctx context.Context) bool {
	readOnly, _ := ctx.Value(readOnlyServiceTokenContextKey{}).(bool)
	return readOnly
}

// RedactValues returns a copy of values with every value replaced by the
// shared redaction placeholder. The keys stay, so a read-only client still sees
// which options or headers are configured without learning their values.
func RedactValues(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	redacted := make(map[string]string, len(values))
	for key := range values {
		redacted[key] = settings.RedactedSensitiveValue
	}
	return redacted
}
