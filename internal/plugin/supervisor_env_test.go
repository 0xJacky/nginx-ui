package plugin

import (
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

func TestSupervisorEnvDropsHostOnlyVariables(t *testing.T) {
	t.Setenv("LEGO_DISABLE_CNAME_SUPPORT", "true")
	t.Setenv("NGINX_UI_NODE_SECRET", "hidden")
	t.Setenv("PLUGIN_ENV_PROBE", "kept")

	s := NewSupervisor(SupervisorConfig{
		PluginID:    "com.example.dns",
		DataDir:     t.TempDir(),
		HostVersion: "2.7.0",
	})

	env := s.env("")
	for _, entry := range env {
		for _, hidden := range []string{"LEGO_DISABLE_CNAME_SUPPORT=", "NGINX_UI_NODE_SECRET="} {
			if strings.HasPrefix(entry, hidden) {
				t.Fatalf("the plugin inherited %q", entry)
			}
		}
	}
	if !slices.Contains(env, "PLUGIN_ENV_PROBE=kept") {
		t.Fatal("the plugin lost an unrelated environment variable")
	}
	// The variables the host sets for the plugin itself share the prefix
	// and must still be there.
	if !slices.Contains(env, EnvPluginID+"=com.example.dns") {
		t.Fatal("the plugin id is missing from the environment")
	}
}

// envValue returns the last value set for key, and whether it is set.
func envValue(env []string, key string) (string, bool) {
	value, found := "", false
	for _, entry := range env {
		if k, v, ok := strings.Cut(entry, "="); ok && k == key {
			value, found = v, true
		}
	}
	return value, found
}

func proxyTestSupervisor(t *testing.T, proxy string, permissions ...string) *Supervisor {
	t.Helper()
	previous := hostHTTPProxy
	hostHTTPProxy = func() string { return proxy }
	t.Cleanup(func() { hostHTTPProxy = previous })

	return NewSupervisor(SupervisorConfig{
		PluginID:    "com.example.proxy",
		DataDir:     t.TempDir(),
		HostVersion: "2.7.0",
		Permissions: permissions,
	})
}

func TestSupervisorEnvHandsTheProxyToPluginsWithNetwork(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://inherited.example:1")
	t.Setenv("https_proxy", "http://inherited.example:2")
	t.Setenv("NO_PROXY", "internal.example")

	s := proxyTestSupervisor(t, "http://user:pw@proxy.example:8080", protocol.PermissionNetwork)
	env := s.env("")

	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if got, _ := envValue(env, key); got != "http://user:pw@proxy.example:8080" {
			t.Fatalf("%s = %q, want the configured proxy", key, got)
		}
	}
	for _, key := range []string{"NO_PROXY", "no_proxy"} {
		if got, _ := envValue(env, key); got != "localhost,127.0.0.1,::1,internal.example" {
			t.Fatalf("%s = %q", key, got)
		}
	}

	// The inherited values are replaced, not repeated.
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, "HTTP_PROXY=") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("HTTP_PROXY appears %d times", count)
	}
}

func TestSupervisorEnvKeepsTheProxyOutOfPluginsWithoutNetwork(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://inherited.example:2")
	t.Setenv("no_proxy", "internal.example")

	s := proxyTestSupervisor(t, "http://proxy.example:8080", protocol.PermissionKV)
	env := s.env("")

	for _, key := range proxyEnv {
		if value, found := envValue(env, key); found {
			t.Fatalf("%s = %q reached a plugin without the network permission", key, value)
		}
	}
	if _, found := envValue(env, EnvPluginID); !found {
		t.Fatal("the plugin id is missing from the environment")
	}
}

func TestSupervisorEnvWithoutAConfiguredProxyLeavesTheEnvironmentAlone(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://inherited.example:2")
	t.Setenv("NO_PROXY", "internal.example")

	s := proxyTestSupervisor(t, "", protocol.PermissionNetwork)
	env := s.env("")

	if got, _ := envValue(env, "HTTPS_PROXY"); got != "http://inherited.example:2" {
		t.Fatalf("HTTPS_PROXY = %q, want the inherited value", got)
	}
	if got, _ := envValue(env, "NO_PROXY"); got != "internal.example" {
		t.Fatalf("NO_PROXY = %q, want the inherited value", got)
	}
}

func TestMergeNoProxy(t *testing.T) {
	if got := mergeNoProxy("localhost,127.0.0.1", " a.example, localhost,,b.example"); got != "localhost,127.0.0.1,a.example,b.example" {
		t.Fatalf("merged = %q", got)
	}
}

func httpSecretSupervisor(t *testing.T, listen string) *Supervisor {
	t.Helper()
	m := &protocol.Manifest{
		ID:           "com.example.http",
		Capabilities: []string{protocol.CapabilityHTTP},
		HTTP:         &protocol.ManifestHTTP{Listen: listen},
	}
	return NewSupervisor(SupervisorConfig{PluginID: m.ID, DataDir: t.TempDir(), Manifest: m})
}

func TestSupervisorHTTPSecretIsRandomPerSpawnAndOnlyForListeners(t *testing.T) {
	t.Setenv(EnvPluginHTTPSecret, "inherited")

	s := httpSecretSupervisor(t, "unix")
	first, err := s.newHTTPSecret()
	require.NoError(t, err)
	second, err := s.newHTTPSecret()
	require.NoError(t, err)

	raw, err := base64.RawURLEncoding.DecodeString(first)
	require.NoError(t, err, "the secret is url safe")
	assert.GreaterOrEqual(t, len(raw), 32)
	assert.NotEqual(t, first, second, "every spawn gets a new secret")

	env := s.env(first)
	assert.Equal(t, 1, strings.Count(strings.Join(env, "\n"), EnvPluginHTTPSecret+"="))
	assert.Contains(t, env, EnvPluginHTTPSecret+"="+first)

	// Plugins without a listener get none, and nothing is inherited.
	rpc := httpSecretSupervisor(t, "rpc")
	none, err := rpc.newHTTPSecret()
	require.NoError(t, err)
	assert.Empty(t, none)
	_, found := envValue(rpc.env(none), EnvPluginHTTPSecret)
	assert.False(t, found)

	plain := proxyTestSupervisor(t, "")
	none, err = plain.newHTTPSecret()
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestSupervisorHTTPSecretFollowsTheProcess(t *testing.T) {
	s := httpSecretSupervisor(t, "unix")
	_, ok := s.HTTPSecret()
	assert.False(t, ok, "no process, no secret")

	s.proc = &process{httpSecret: "abc"}
	got, ok := s.HTTPSecret()
	assert.True(t, ok)
	assert.Equal(t, "abc", got)

	s.proc = &process{}
	_, ok = s.HTTPSecret()
	assert.False(t, ok, "a process without a listener has none")
}

func TestSupervisorEnvMarksDemoHosts(t *testing.T) {
	// An inherited value never reaches the plugin, only the host setting does.
	t.Setenv(EnvDemo, "true")
	previous := hostDemo
	t.Cleanup(func() { hostDemo = previous })

	s := NewSupervisor(SupervisorConfig{
		PluginID:    "com.example.demo",
		DataDir:     t.TempDir(),
		HostVersion: "2.7.0",
	})

	hostDemo = func() bool { return false }
	_, found := envValue(s.env(""), EnvDemo)
	assert.False(t, found)

	hostDemo = func() bool { return true }
	got, found := envValue(s.env(""), EnvDemo)
	assert.True(t, found)
	assert.Equal(t, "1", got)
}
