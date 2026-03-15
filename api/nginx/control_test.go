package nginx

import (
	"testing"

	internalnginx "github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/stretchr/testify/assert"
)

func TestBuildNamespaceTestConfigResponseIncludesSandboxFields(t *testing.T) {
	response := buildNamespaceTestConfigResponse(9, internalnginx.TestConfigResult{
		Message:       "sandbox failed",
		Level:         internalnginx.Error,
		TestScope:     internalnginx.TestScopeNamespaceSandbox,
		SandboxStatus: internalnginx.SandboxStatusFailed,
		ErrorCategory: internalnginx.ErrorCategoryMissingInclude,
	})

	assert.Equal(t, uint64(9), response["namespace_id"])
	assert.Equal(t, "sandbox failed", response["message"])
	assert.Equal(t, internalnginx.TestScopeNamespaceSandbox, response["test_scope"])
	assert.Equal(t, internalnginx.SandboxStatusFailed, response["sandbox_status"])
	assert.Equal(t, internalnginx.ErrorCategoryMissingInclude, response["error_category"])
}
