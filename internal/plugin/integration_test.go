package plugin

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationDirEnv points at an extracted plugin package built for this
// platform. The test is skipped when it is unset, so CI does not need the
// plugin repository.
const integrationDirEnv = "NGINX_UI_PLUGIN_INTEGRATION_DIR"

type integrationBackend struct{}

func (integrationBackend) Log(string, protocol.HostLogParams) {}
func (integrationBackend) KVGet(string, string) (json.RawMessage, bool, error) {
	return nil, false, nil
}
func (integrationBackend) KVSet(string, string, json.RawMessage) error { return nil }
func (integrationBackend) KVDelete(string, string) error               { return nil }
func (integrationBackend) KVList(string, string) ([]string, error)     { return nil, nil }
func (integrationBackend) SettingsGet(string) (map[string]any, error)  { return map[string]any{}, nil }
func (integrationBackend) Locale() string                              { return "en" }
func (integrationBackend) CredentialsGet(string, string, string) (*protocol.HostCredentialsGetResult, error) {
	return nil, nil
}
func (integrationBackend) CronRegister(string, protocol.HostCronRegisterParams) error { return nil }
func (integrationBackend) CronUnregister(string, string) error                        { return nil }
func (integrationBackend) Notify(string, protocol.HostNotifyParams) error             { return nil }
func (integrationBackend) MetricsSnapshot() (any, error)                              { return nil, nil }

// integrationOutcome is what one transport answered, compared across the
// transports at the end.
type integrationOutcome struct {
	options  protocol.DNS01OptionsResult
	validate *protocol.Error
	unknown  int
}

// TestIntegrationRealPlugin drives a real plugin binary through the handshake
// and the dns01 capability methods that need no network, once per transport,
// and checks both transports answer the same.
func TestIntegrationRealPlugin(t *testing.T) {
	dir := os.Getenv(integrationDirEnv)
	if dir == "" {
		t.Skipf("%s is not set", integrationDirEnv)
	}

	manifest, err := LoadManifest(dir)
	require.NoError(t, err)
	require.NoError(t, ValidateManifest(manifest))
	argv, err := ResolveExecutable(manifest, dir)
	require.NoError(t, err)

	outcomes := map[string]integrationOutcome{}
	for _, transport := range []string{protocol.TransportStdio, protocol.TransportGRPC} {
		t.Run(transport, func(t *testing.T) {
			outcome, ok := runIntegration(t, dir, manifest, argv, transport)
			if ok {
				outcomes[transport] = outcome
			}
		})
	}

	stdio, okStdio := outcomes[protocol.TransportStdio]
	grpc, okGRPC := outcomes[protocol.TransportGRPC]
	if okStdio && okGRPC {
		assert.Equal(t, stdio, grpc, "both transports must answer the same")
	}
}

// runIntegration runs the plugin with capability calls on one transport. ok
// is false when the transport was skipped.
func runIntegration(t *testing.T, dir string, manifest *protocol.Manifest, argv []string, transport string) (integrationOutcome, bool) {
	sup := NewSupervisor(SupervisorConfig{
		PluginID:    manifest.ID,
		Dir:         dir,
		DataDir:     t.TempDir(),
		Manifest:    manifest,
		Argv:        argv,
		HostVersion: "2.7.0",
		Locale:      "en",
		Settings:    map[string]any{},
		Permissions: manifest.Permissions,
		Lifecycle:   protocol.LifecycleOnDemand,
		IdleTimeout: time.Minute,
		HostHandlers: func(conn *jsonrpc.Conn) {
			RegisterHostHandlers(conn, manifest.ID, manifest.Permissions, integrationBackend{})
		},
	})
	if transport == protocol.TransportStdio {
		// The Go SDK serves gRPC by default; this keeps the plugin on stdio.
		sup.extraEnv = []string{"NGINX_UI_PLUGIN_DISABLE_GRPC=1"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	caller, release, err := sup.Acquire(ctx)
	require.NoError(t, err)
	defer release()
	require.Equal(t, StateRunning, sup.State())

	init, ok := sup.InitializeResult()
	require.True(t, ok)
	require.Equal(t, protocol.APIVersion, init.APIVersion)
	require.Contains(t, init.Capabilities, protocol.CapabilityDNS01)

	if transport == protocol.TransportGRPC && !slices.Contains(init.Transports, protocol.TransportGRPC) {
		require.NoError(t, sup.Stop(ctx))
		t.Skipf("the plugin does not advertise grpc (transports %v)", init.Transports)
		return integrationOutcome{}, false
	}
	require.Equal(t, transport, sup.Transport())

	var outcome integrationOutcome
	require.NoError(t, caller.Call(ctx, protocol.MethodDNS01Options, protocol.DNS01OptionsParams{
		Provider: "manual", Config: map[string]string{},
	}, &outcome.options))
	require.Greater(t, outcome.options.PropagationTimeoutSeconds, 0)
	require.Greater(t, outcome.options.PollingIntervalSeconds, 0)

	err = caller.Call(ctx, protocol.MethodDNS01Validate, protocol.DNS01ValidateParams{
		Provider: "cloudflare", Config: map[string]string{},
	}, nil)
	require.Error(t, err)
	pErr, ok := jsonrpc.AsProtocolError(err)
	require.True(t, ok, "expected a protocol error, got %v", err)
	require.Equal(t, protocol.CodeInvalidConfig, pErr.Code)
	outcome.validate = pErr

	err = caller.Call(ctx, "dns01.nonexistent", nil, nil)
	require.True(t, jsonrpc.IsMethodNotFound(err), "got %v", err)
	outcome.unknown = protocol.CodeMethodNotFound

	// The capability calls above must not have knocked gRPC over.
	require.Equal(t, transport, sup.Transport())

	require.NoError(t, sup.Stop(ctx))
	require.Equal(t, StateStopped, sup.State())
	return outcome, true
}
