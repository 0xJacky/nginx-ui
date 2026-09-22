package plugin

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge/bridgetest"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// Fake plugin modes that serve the gRPC transport next to stdio.
const (
	// pluginModeGRPC serves gRPC and reports the socket in rpc_socket.
	pluginModeGRPC = "grpc"
	// pluginModeGRPCDefault serves gRPC on <data dir>/rpc.sock without
	// reporting it.
	pluginModeGRPCDefault = "grpcdefault"
	// pluginModeGRPCBroken advertises gRPC but nothing listens.
	pluginModeGRPCBroken = "grpcbroken"
)

// Stdio methods the gRPC modes add for the tests to look inside the process.
const (
	testMethodGRPCCalls = "test.grpc_calls"
	testMethodGRPCStop  = "test.grpc_stop"
)

// testPluginGRPC is the gRPC side of the fake plugin process.
var testPluginGRPC struct {
	mu     sync.Mutex
	server *grpc.Server
	calls  atomic.Int64
}

func isGRPCMode(mode string) bool {
	return mode == pluginModeGRPC || mode == pluginModeGRPCDefault || mode == pluginModeGRPCBroken ||
		mode == pluginModeAllCaps
}

// extendTestInitialize opens the gRPC listener of the gRPC modes and
// advertises it in the handshake. It runs inside the fake plugin process.
func extendTestInitialize(mode string, result *protocol.InitializeResult) {
	if !isGRPCMode(mode) {
		return
	}
	result.Transports = []string{protocol.TransportStdio, protocol.TransportGRPC}

	dataDir := os.Getenv(EnvPluginDataDir)
	switch mode {
	case pluginModeGRPCBroken:
		result.RPCSocket = filepath.Join(dataDir, "nothing-listens-here.sock")
		return
	case pluginModeGRPCDefault:
		_ = os.MkdirAll(dataDir, 0o700)
		startTestGRPC(filepath.Join(dataDir, grpcbridge.DefaultSocketName))
		return
	}

	socket := filepath.Join(dataDir, grpcbridge.DefaultSocketName)
	if len(socket) > 100 || os.MkdirAll(dataDir, 0o700) != nil {
		dir, err := os.MkdirTemp("", "nuit")
		if err != nil {
			return
		}
		socket = filepath.Join(dir, grpcbridge.DefaultSocketName)
	}
	if startTestGRPC(socket) {
		result.RPCSocket = socket
	}
}

func startTestGRPC(socket string) bool {
	lis, err := net.Listen("unix", socket)
	if err != nil {
		return false
	}
	server := bridgetest.NewServer(func(_ context.Context, method string, params json.RawMessage) (any, error) {
		testPluginGRPC.calls.Add(1)
		if method == protocol.MethodPing {
			return protocol.EmptyResult{}, nil
		}
		if result, err, ok := testPluginCapability(method, params); ok {
			return result, err
		}
		return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method " + method}
	})
	testPluginGRPC.mu.Lock()
	testPluginGRPC.server = server
	testPluginGRPC.mu.Unlock()
	go func() { _ = server.Serve(lis) }()
	return true
}

// testPluginCapability answers the dns01 methods of the gRPC modes the same
// way on both transports.
func testPluginCapability(method string, params json.RawMessage) (any, error, bool) {
	switch method {
	case protocol.MethodDNS01Options:
		var decoded protocol.DNS01OptionsParams
		if len(params) > 0 {
			if err := json.Unmarshal(params, &decoded); err != nil {
				return nil, &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}, true
			}
		}
		return protocol.DNS01OptionsResult{PropagationTimeoutSeconds: 90, PollingIntervalSeconds: 3}, nil, true
	case protocol.MethodDNS01Validate:
		return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "TOKEN is required", Data: protocol.InvalidConfigData{Field: "TOKEN"}}, true
	default:
		return testPluginNewCapability(method, params)
	}
}

// handleTestPluginMethod serves the stdio methods only the gRPC modes know.
func handleTestPluginMethod(mode, method string, params json.RawMessage) (any, error, bool) {
	if !isGRPCMode(mode) {
		return nil, nil, false
	}
	switch method {
	case testMethodGRPCCalls:
		return map[string]int64{"calls": testPluginGRPC.calls.Load()}, nil, true
	case testMethodGRPCStop:
		testPluginGRPC.mu.Lock()
		server := testPluginGRPC.server
		testPluginGRPC.mu.Unlock()
		if server != nil {
			server.Stop()
		}
		return protocol.EmptyResult{}, nil, true
	}
	return testPluginCapability(method, params)
}

// pluginAnswer replies with a result or with the error object of err.
func pluginAnswer(id json.RawMessage, result any, err error) {
	if err == nil {
		pluginReply(id, result)
		return
	}
	perr, ok := err.(*protocol.Error)
	if !ok {
		perr = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
	}
	if len(id) == 0 {
		return
	}
	pluginWrite(map[string]any{"jsonrpc": "2.0", "id": id, "error": perr})
}

// shortDataDir returns a data directory whose socket path fits every
// platform's limit.
func shortDataDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "nuid")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// grpcCalls asks the fake plugin, over stdio, how many gRPC calls it served.
func grpcCalls(t *testing.T, caller interface {
	Call(context.Context, string, any, any) error
}) int64 {
	t.Helper()
	var out struct {
		Calls int64 `json:"calls"`
	}
	require.NoError(t, caller.Call(context.Background(), testMethodGRPCCalls, nil, &out))
	return out.Calls
}

func TestSupervisorRoutesCapabilityCallsOverGRPC(t *testing.T) {
	dataDir := shortDataDir(t)
	sup := newTestSupervisor(t, pluginModeGRPC, func(cfg *SupervisorConfig) { cfg.DataDir = dataDir })
	require.NoError(t, sup.Start(context.Background()))
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	assert.Equal(t, protocol.TransportGRPC, sup.Transport())
	init, ok := sup.InitializeResult()
	require.True(t, ok)
	assert.Equal(t, filepath.Join(dataDir, grpcbridge.DefaultSocketName), init.RPCSocket)

	client, err := sup.Client()
	require.NoError(t, err)

	// The handshake probe was the only gRPC call so far.
	assert.EqualValues(t, 1, grpcCalls(t, client))

	var options protocol.DNS01OptionsResult
	require.NoError(t, client.Call(context.Background(), protocol.MethodDNS01Options, protocol.DNS01OptionsParams{Provider: "test"}, &options))
	assert.Equal(t, 90, options.PropagationTimeoutSeconds)
	assert.EqualValues(t, 2, grpcCalls(t, client), "dns01.options must travel over gRPC")

	// Errors look exactly like their stdio counterpart.
	err = client.Call(context.Background(), protocol.MethodDNS01Validate, protocol.DNS01ValidateParams{Provider: "test"}, nil)
	perr, ok := err.(*protocol.Error)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "TOKEN is required", Data: map[string]any{"field": "TOKEN"}}, perr)
	assert.EqualValues(t, 3, grpcCalls(t, client))

	// Lifecycle and plugin private methods stay on stdio.
	require.NoError(t, sup.Configure(context.Background(), map[string]any{"a": "b"}))
	require.NoError(t, client.Call(context.Background(), protocol.MethodPing, nil, nil))
	var echo map[string]string
	require.NoError(t, client.Call(context.Background(), "echo.hello", map[string]string{"text": "hi"}, &echo))
	assert.Equal(t, "HI", echo["text"])
	assert.EqualValues(t, 3, grpcCalls(t, client))

	require.NoError(t, sup.Stop(context.Background()))
	assert.Equal(t, "", sup.Transport())
}

func TestSupervisorFallsBackToStdioWhenGRPCGoesAway(t *testing.T) {
	dataDir := shortDataDir(t)
	sup := newTestSupervisor(t, pluginModeGRPC, func(cfg *SupervisorConfig) { cfg.DataDir = dataDir })
	require.NoError(t, sup.Start(context.Background()))
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })
	require.Equal(t, protocol.TransportGRPC, sup.Transport())

	client, err := sup.Client()
	require.NoError(t, err)
	require.NoError(t, client.Call(context.Background(), testMethodGRPCStop, nil, nil))

	// The call still succeeds, over stdio now.
	var options protocol.DNS01OptionsResult
	require.NoError(t, client.Call(context.Background(), protocol.MethodDNS01Options, protocol.DNS01OptionsParams{Provider: "test"}, &options))
	assert.Equal(t, 90, options.PropagationTimeoutSeconds)

	assert.Eventually(t, func() bool { return sup.Transport() == protocol.TransportStdio }, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, StateRunning, sup.State())
}

func TestSupervisorStaysOnStdioWhenTheGRPCDialFails(t *testing.T) {
	sup := newTestSupervisor(t, pluginModeGRPCBroken, func(cfg *SupervisorConfig) { cfg.DataDir = shortDataDir(t) })
	sup.grpcDialTimeout = 2 * time.Second
	started := time.Now()
	require.NoError(t, sup.Start(context.Background()))
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	assert.Less(t, time.Since(started), 5*time.Second)
	assert.Equal(t, protocol.TransportStdio, sup.Transport())

	client, err := sup.Client()
	require.NoError(t, err)
	var options protocol.DNS01OptionsResult
	require.NoError(t, client.Call(context.Background(), protocol.MethodDNS01Options, protocol.DNS01OptionsParams{Provider: "test"}, &options))
	assert.Equal(t, 90, options.PropagationTimeoutSeconds)
}

func TestSupervisorDialsTheDefaultSocket(t *testing.T) {
	sup := newTestSupervisor(t, pluginModeGRPCDefault, func(cfg *SupervisorConfig) { cfg.DataDir = shortDataDir(t) })
	require.NoError(t, sup.Start(context.Background()))
	t.Cleanup(func() { _ = sup.Stop(context.Background()) })

	init, _ := sup.InitializeResult()
	assert.Empty(t, init.RPCSocket)
	assert.Equal(t, protocol.TransportGRPC, sup.Transport())
}

func TestSupervisorReportsStdioForAStdioPlugin(t *testing.T) {
	sup := newTestSupervisor(t, pluginModeNormal, nil)
	assert.Equal(t, "", sup.Transport())
	require.NoError(t, sup.Start(context.Background()))
	assert.Equal(t, protocol.TransportStdio, sup.Transport())
	require.NoError(t, sup.Stop(context.Background()))
	assert.Equal(t, "", sup.Transport())
}

// casesByKey indexes a report by transport, rule and name.
func casesByKey(report *ConformanceReport) map[string]CaseResult {
	out := make(map[string]CaseResult, len(report.Cases))
	for _, c := range report.Cases {
		out[c.Transport+"|"+c.Rule+":"+c.Name] = c
	}
	return out
}

func TestConformanceRunsBothTransports(t *testing.T) {
	usePluginProcesses(t, pluginModeGRPC)

	dir := t.TempDir()
	writePluginDir(t, dir, pluginManifest("official.conformance-grpc"))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// The default runs both transports when the plugin advertises gRPC.
	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 50 * time.Second})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)

	byKey := casesByKey(report)
	for _, key := range []string{
		"stdio|LIFE-8:plugin.ping",
		"grpc|WIRE-11:grpc transport",
		"grpc|LIFE-8:plugin.ping",
		"grpc|WIRE-6:unknown method",
		"grpc|WIRE-6:invalid params",
		"grpc|WIRE-4:concurrent pings",
		"grpc|DNS01-10:dns01.options",
		"grpc|DNS01-9:dns01.validate",
		"|TRANSPORT-1:identical results",
	} {
		c, ok := byKey[key]
		if assert.True(t, ok, "missing case %s in %+v", key, report.Cases) {
			assert.Equal(t, StatusPass, c.Status, "%s: %s", key, c.Message)
		}
	}
	_, stdioOnly := byKey["grpc|WIRE-2:unanswered notification"]
	assert.False(t, stdioOnly, "notifications are a stdio case")

	// Asking for one transport runs only that one.
	report, err = Conformance(ctx, dir, ConformanceOptions{Timeout: 50 * time.Second, Transport: TransportFlagStdio})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)
	for _, c := range report.Cases {
		assert.NotEqual(t, protocol.TransportGRPC, c.Transport, "%+v", c)
		assert.NotEqual(t, "TRANSPORT-1", c.Rule)
	}
}

func TestConformanceFailsGRPCForAStdioPlugin(t *testing.T) {
	usePluginProcesses(t, pluginModeNormal)

	dir := t.TempDir()
	writePluginDir(t, dir, pluginManifest("official.conformance-stdio"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := Conformance(ctx, dir, ConformanceOptions{Timeout: 25 * time.Second, Transport: TransportFlagGRPC})
	require.NoError(t, err)
	assert.False(t, report.Passed())
	c, ok := casesByKey(report)["grpc|WIRE-11:grpc transport"]
	require.True(t, ok, "%+v", report.Cases)
	assert.Equal(t, StatusFail, c.Status)

	// Without an explicit request a stdio plugin only runs the stdio cases.
	report, err = Conformance(ctx, dir, ConformanceOptions{Timeout: 25 * time.Second})
	require.NoError(t, err)
	assert.True(t, report.Passed(), "%+v", report.Cases)
	for _, c := range report.Cases {
		assert.NotEqual(t, protocol.TransportGRPC, c.Transport, "%+v", c)
	}
}
