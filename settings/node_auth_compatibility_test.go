package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectLegacyNodeAuthCompatibility(t *testing.T) {
	t.Run("fresh configuration defaults to paired authentication", func(t *testing.T) {
		path := writeNodeAuthCompatibilityConfig(t, "[node]\nSecret =\n")

		secretConfigured, authExplicit, mcpAuthExplicit := detectLegacyNodeAuthCompatibility(path)

		assert.False(t, secretConfigured)
		assert.False(t, authExplicit)
		assert.False(t, mcpAuthExplicit)
	})

	t.Run("existing secret enables compatibility when switches are absent", func(t *testing.T) {
		path := writeNodeAuthCompatibilityConfig(t, "[node]\nSecret = existing-secret\n")

		secretConfigured, authExplicit, mcpAuthExplicit := detectLegacyNodeAuthCompatibility(path)

		assert.True(t, secretConfigured)
		assert.False(t, authExplicit)
		assert.False(t, mcpAuthExplicit)
	})

	t.Run("explicit switches are preserved", func(t *testing.T) {
		path := writeNodeAuthCompatibilityConfig(t, "[node]\nSecret = existing-secret\nLegacyAuthEnabled = false\nLegacyMCPAuthEnabled = false\n")

		secretConfigured, authExplicit, mcpAuthExplicit := detectLegacyNodeAuthCompatibility(path)

		assert.True(t, secretConfigured)
		assert.True(t, authExplicit)
		assert.True(t, mcpAuthExplicit)
	})

	t.Run("deprecated environment secret retains compatibility", func(t *testing.T) {
		path := writeNodeAuthCompatibilityConfig(t, "[node]\nSecret =\n")
		t.Setenv(EnvPrefix+"SERVER_NODE_SECRET", "existing-secret")

		secretConfigured, _, _ := detectLegacyNodeAuthCompatibility(path)

		assert.True(t, secretConfigured)
	})

	t.Run("environment switches are explicit even when false", func(t *testing.T) {
		path := writeNodeAuthCompatibilityConfig(t, "[node]\nSecret = existing-secret\n")
		t.Setenv(EnvPrefix+"NODE_LEGACY_AUTH_ENABLED", "false")
		t.Setenv(EnvPrefix+"NODE_LEGACY_MCP_AUTH_ENABLED", "false")

		_, authExplicit, mcpAuthExplicit := detectLegacyNodeAuthCompatibility(path)

		assert.True(t, authExplicit)
		assert.True(t, mcpAuthExplicit)
	})
}

func writeNodeAuthCompatibilityConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.ini")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
