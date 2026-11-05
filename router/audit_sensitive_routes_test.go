//go:build unembed

package router

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/api/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cosyRouter "github.com/uozi-tech/cosy/router"
)

// TestAuditSensitiveRoutesMatchRegisteredRoutes keeps the audit redaction
// table in step with the router, so a renamed route cannot silently drop out
// of redaction.
func TestAuditSensitiveRoutesMatchRegisteredRoutes(t *testing.T) {
	debugRouteCredentials(t)

	registered := make(map[string]struct{})
	for _, route := range cosyRouter.GetEngine().Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}
	sensitive := audit.SensitiveRoutes()
	require.NotEmpty(t, sensitive)
	for _, route := range sensitive {
		_, ok := registered[route]
		assert.True(t, ok, "audit redaction rule %q does not match a registered route", route)
	}
	configFiles := audit.ConfigFileRoutes()
	require.NotEmpty(t, configFiles)
	for _, route := range configFiles {
		_, ok := registered[route]
		assert.True(t, ok, "audit config file rule %q does not match a registered route", route)
	}
}
