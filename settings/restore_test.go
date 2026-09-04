package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/ini.v1"
)

func writeRestoreConfigFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestBuildRestoreConfigPreservesEveryProtectedField(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", `[app]
JwtSecret = destination-jwt
[auth]
IPWhiteList = 192.0.2.1
IPWhiteList = 192.0.2.2
TrustedProxies = 127.0.0.1
TrustedProxies = 10.0.0.0/8
[crypto]
Secret = destination-crypto
[cluster]
Node = https://destination.example?name=destination&node_secret=destination-secret
[node]
Secret = destination-node
InstanceID = destination-instance
SkipInstallation = true
Demo = true
[nginx]
ConfigDir = /destination/nginx
[server]
Port = 9000
`)
	backupPath := writeRestoreConfigFixture(t, "backup.ini", `[app]
JwtSecret = source-jwt
[auth]
IPWhiteList = 198.51.100.1
TrustedProxies = 0.0.0.0/0
[crypto]
Secret = source-crypto
[cluster]
Node = https://source.example?name=source&node_secret=source-secret
[node]
Secret = source-node
InstanceID = source-instance
SkipInstallation = false
Demo = false
[nginx]
ConfigDir = /source/nginx
[server]
Port = 9443
`)

	merged, skipped, err := BuildRestoreConfig(backupPath, currentPath, true)
	require.NoError(t, err)

	config, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true}, merged)
	require.NoError(t, err)
	assert.Equal(t, "destination-jwt", config.Section("app").Key("JwtSecret").String())
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, config.Section("auth").Key("IPWhiteList").ValueWithShadows())
	assert.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, config.Section("auth").Key("TrustedProxies").ValueWithShadows())
	assert.Equal(t, "destination-crypto", config.Section("crypto").Key("Secret").String())
	assert.Contains(t, config.Section("cluster").Key("Node").String(), "destination-secret")
	assert.Equal(t, "destination-node", config.Section("node").Key("Secret").String())
	assert.Equal(t, "destination-instance", config.Section("node").Key("InstanceID").String())
	assert.Equal(t, "true", config.Section("node").Key("SkipInstallation").String())
	assert.Equal(t, "true", config.Section("node").Key("Demo").String())
	assert.Equal(t, "/destination/nginx", config.Section("nginx").Key("ConfigDir").String())
	assert.Equal(t, "9443", config.Section("server").Key("Port").String())
	assert.Contains(t, skipped, "app.JwtSecret")
	assert.Contains(t, skipped, "auth.TrustedProxies")
	assert.Contains(t, skipped, "crypto.Secret")
	assert.Contains(t, skipped, "cluster.Node")
	assert.Contains(t, skipped, "node.Secret")
	assert.Contains(t, skipped, "node.InstanceID")
	assert.Contains(t, skipped, "nginx.ConfigDir")
}

func TestBuildRestoreConfigAllowsFullTrustedRestore(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", "[app]\nJwtSecret=destination\n")
	backupPath := writeRestoreConfigFixture(t, "backup.ini", "[app]\nJwtSecret=source\n[server]\nPort=9443\n")

	merged, skipped, err := BuildRestoreConfig(backupPath, currentPath, false)
	require.NoError(t, err)
	assert.Empty(t, skipped)

	config, err := ini.Load(merged)
	require.NoError(t, err)
	assert.Equal(t, "source", config.Section("app").Key("JwtSecret").String())
}

func TestBuildRestoreConfigDoesNotImportProtectedValueMissingFromCurrentConfig(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", "[server]\nPort=9000\n")
	backupPath := writeRestoreConfigFixture(t, "backup.ini", "[node]\nInstanceID=source-instance\n[server]\nPort=9443\n")

	merged, skipped, err := BuildRestoreConfig(backupPath, currentPath, true)
	require.NoError(t, err)

	config, err := ini.Load(merged)
	require.NoError(t, err)
	assert.False(t, config.Section("node").HasKey("InstanceID"))
	assert.Equal(t, "9443", config.Section("server").Key("Port").String())
	assert.Contains(t, skipped, "node.InstanceID")
}

func TestBuildRestoreConfigRejectsInvalidTypedValue(t *testing.T) {
	currentPath := writeRestoreConfigFixture(t, "current.ini", "[app]\nJwtSecret=destination\n")
	backupPath := writeRestoreConfigFixture(t, "backup.ini", "[server]\nPort=not-a-number\n")

	_, _, err := BuildRestoreConfig(backupPath, currentPath, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server")
}
