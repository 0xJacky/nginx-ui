package settings

import (
	"crypto/rand"
	"testing"

	"aead.dev/minisign"
	appsettings "github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePluginSettingsChecksTrustedPublishers(t *testing.T) {
	public, _, err := minisign.GenerateKey(rand.Reader)
	require.NoError(t, err)
	encoded, err := public.MarshalText()
	require.NoError(t, err)
	key := string(encoded)

	assert.NoError(t, validatePluginSettings(&appsettings.Plugin{}))
	assert.NoError(t, validatePluginSettings(&appsettings.Plugin{TrustedPublicKeys: []string{key + " Example Labs"}}))

	for name, entries := range map[string][]string{
		"not a key": {"RWnot-a-key Example"},
		"duplicate": {key, key + " Again"},
	} {
		assert.Error(t, validatePluginSettings(&appsettings.Plugin{TrustedPublicKeys: entries}), name)
	}
}

func TestPluginSettingsResponseResolvesTheDirectory(t *testing.T) {
	original := *appsettings.PluginSettings
	defer func() { *appsettings.PluginSettings = original }()

	appsettings.PluginSettings.Dir = ""
	response := buildPluginSettingsResponse()
	assert.NotEmpty(t, response["dir"])
	assert.Contains(t, response, "resource_limits_supported")
}
