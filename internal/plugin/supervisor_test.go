package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPluginModeEnv turns the test binary into a plugin process. TestMain
// checks it before any test runs, which gives the supervisor a real child
// process to talk to without shipping a second binary.
const testPluginModeEnv = "PLUGIN_TEST_MODE"

// Plugin behaviours the supervisor tests need.
const (
	pluginModeNormal     = "normal"
	pluginModeCrash      = "crash"
	pluginModeStubborn   = "stubborn"
	pluginModeWrongCaps  = "wrongcaps"
	pluginModeBadAPI     = "badapi"
	pluginModeDeaf       = "deaf"
	pluginModeRemotePipe = "remotepipe"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(testPluginModeEnv); mode != "" {
		runTestPlugin(mode)
		return
	}
	os.Exit(m.Run())
}

// runTestPlugin is a minimal NDJSON JSON-RPC peer: one JSON object per line on
// stdin and stdout, human text on stderr.
func runTestPlugin(mode string) {
	fmt.Fprintf(os.Stderr, "test plugin started in %s mode\n", mode)

	reader := bufio.NewScanner(os.Stdin)
	reader.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for reader.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(reader.Bytes(), &msg); err != nil {
			continue
		}

		switch msg.Method {
		case protocol.MethodInitialize:
			capabilities := []string{protocol.CapabilityDNS01}
			if mode == pluginModeWrongCaps {
				capabilities = []string{protocol.CapabilityHTTP}
			}
			if mode == pluginModeAllCaps {
				capabilities = allTestCapabilities
			}
			if mode == pluginModeLogSink || mode == pluginModeLogSinkStdio {
				capabilities = []string{protocol.CapabilityLogSink}
			}
			apiVersion := protocol.APIVersion
			if mode == pluginModeBadAPI {
				apiVersion = protocol.APIVersion + 98
			}
			result := protocol.InitializeResult{APIVersion: apiVersion, Capabilities: capabilities}
			if mode == pluginModeRemotePipe {
				result.HTTPPipe = `\\attacker\pipe\x`
			}
			extendTestInitialize(mode, &result)
			pluginReply(msg.ID, result)
		case protocol.MethodInitialized:
			if mode == pluginModeCrash {
				// Exit only once the handshake is complete, so the supervisor
				// sees a crash instead of a handshake failure.
				os.Exit(3)
			}
			pluginRequest(9001, protocol.MethodHostLog, protocol.HostLogParams{Level: "info", Message: "hello from the plugin"})
		case protocol.MethodConfigure:
			fmt.Fprintln(os.Stderr, "plugin configured")
			pluginReply(msg.ID, protocol.EmptyResult{})
		case protocol.MethodPing:
			if mode == pluginModeDeaf {
				continue
			}
			pluginReply(msg.ID, protocol.EmptyResult{})
		case protocol.MethodShutdown:
			if mode == pluginModeStubborn {
				continue
			}
			pluginReply(msg.ID, protocol.EmptyResult{})
		case protocol.MethodExit:
			if mode == pluginModeStubborn {
				continue
			}
			os.Exit(0)
		case protocol.MethodEventsOn:
			var notification protocol.EventNotification
			_ = json.Unmarshal(msg.Params, &notification)
			fmt.Fprintf(os.Stderr, "event %s\n", notification.Type)
		case "echo.hello":
			var params struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			pluginReply(msg.ID, map[string]string{"text": strings.ToUpper(params.Text)})
		case "":
			// A response to one of our own requests, nothing to do.
		default:
			if result, err, ok := handleTestPluginMethod(mode, msg.Method, msg.Params); ok {
				pluginAnswer(msg.ID, result, err)
				continue
			}
			pluginReplyError(msg.ID, protocol.CodeMethodNotFound, "unknown method "+msg.Method)
		}
	}

	if mode == pluginModeStubborn {
		// Ignore the closed stdin too and wait to be killed.
		select {}
	}
	os.Exit(0)
}

func pluginWrite(frame map[string]any) {
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	_, _ = os.Stdout.Write(append(data, '\n'))
}

func pluginReply(id json.RawMessage, result any) {
	if len(id) == 0 {
		return
	}
	pluginWrite(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func pluginReplyError(id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		return
	}
	pluginWrite(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   protocol.Error{Code: code, Message: message},
	})
}

func pluginRequest(id int, method string, params any) {
	pluginWrite(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

// testManifest is the manifest the fake plugin matches during the handshake.
func testManifest() *protocol.Manifest {
	return &protocol.Manifest{
		ID:           "official.test",
		Name:         "Test Plugin",
		Version:      "1.0.0",
		APIVersion:   protocol.APIVersion,
		Server:       &protocol.ManifestServer{Command: []string{"plugin"}},
		Capabilities: []string{protocol.CapabilityDNS01},
		DNS01: &protocol.ManifestDNS01{
			Providers: []protocol.DNS01Provider{{Name: "Test", Code: "test", Form: &protocol.DNS01ProviderForm{Fields: []protocol.DNS01ProviderField{{Key: "TEST_TOKEN", Label: "API token", Group: "credential"}}}}},
		},
	}
}

// newTestSupervisor wires a supervisor to the test binary running as a plugin
// and shrinks every timer so the suite stays fast.
func newTestSupervisor(t *testing.T, mode string, adjust func(cfg *SupervisorConfig)) *Supervisor {
	t.Helper()

	executable, err := os.Executable()
	require.NoError(t, err)

	dir := t.TempDir()
	cfg := SupervisorConfig{
		PluginID:    "official.test",
		Dir:         dir,
		DataDir:     filepath.Join(dir, "data"),
		Manifest:    testManifest(),
		Argv:        []string{executable},
		HostVersion: "2.0.0-test",
		Locale:      "en_US",
		Settings:    map[string]any{"token": "value"},
		Permissions: []string{protocol.PermissionKV},
	}
	if adjust != nil {
		adjust(&cfg)
	}

	supervisor := NewSupervisor(cfg)
	supervisor.extraEnv = []string{testPluginModeEnv + "=" + mode}
	supervisor.handshakeTimeout = 10 * time.Second
	supervisor.pingInterval = 250 * time.Millisecond
	supervisor.pingTimeout = 2 * time.Second
	supervisor.shutdownTimeout = 500 * time.Millisecond
	// A race instrumented Go binary needs about a second to shut down, so the
	// exit grace period stays at the production value.
	supervisor.exitTimeout = defaultExitTimeout
	supervisor.idleTimeout = 200 * time.Millisecond
	supervisor.backoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	t.Cleanup(func() { _ = supervisor.Stop(context.Background()) })
	return supervisor
}

func TestSupervisorHandshakeAndCall(t *testing.T) {
	states := make(chan State, 8)
	logs := make(chan protocol.HostLogParams, 4)
	backend := &fakeHostBackend{onLog: func(p protocol.HostLogParams) { logs <- p }}

	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.OnStateChange = func(state State, err error) { states <- state }
		cfg.HostHandlers = func(conn *jsonrpc.Conn) {
			RegisterHostHandlers(conn, "official.test", []string{protocol.PermissionKV}, backend)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, supervisor.Start(ctx))
	assert.Equal(t, StateRunning, supervisor.State())
	assert.NoError(t, supervisor.LastError())

	result, ok := supervisor.InitializeResult()
	require.True(t, ok)
	assert.Equal(t, protocol.APIVersion, result.APIVersion)

	client, err := supervisor.Client()
	require.NoError(t, err)

	var echo struct {
		Text string `json:"text"`
	}
	require.NoError(t, client.Call(ctx, "echo.hello", map[string]string{"text": "ping"}, &echo))
	assert.Equal(t, "PING", echo.Text)

	// A resident plugin hands out the same client without counting acquisitions.
	acquired, release, err := supervisor.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, acquired)
	release()

	// The plugin called back into the host API right after the handshake.
	select {
	case entry := <-logs:
		assert.Equal(t, "hello from the plugin", entry.Message)
	case <-time.After(10 * time.Second):
		t.Fatal("the plugin never called host.log")
	}

	// Unknown methods travel back as JSON-RPC errors.
	err = client.Call(ctx, "does.not.exist", nil, nil)
	assert.True(t, jsonrpc.IsMethodNotFound(err))
	assert.ErrorIs(t, WrapRPCError(err), ErrRPC)
	assert.ErrorContains(t, WrapRPCError(err), "unknown method does.not.exist")

	assert.Eventually(t, func() bool {
		return logsContain(supervisor, "test plugin started in normal mode")
	}, 10*time.Second, 20*time.Millisecond, "the stderr ring buffer stayed empty")

	// A settings change reaches the running plugin.
	require.NoError(t, supervisor.Configure(ctx, map[string]any{"token": "rotated"}))
	assert.Eventually(t, func() bool {
		return logsContain(supervisor, "plugin configured")
	}, 10*time.Second, 20*time.Millisecond, "plugin.configure never arrived")

	require.NoError(t, supervisor.Stop(ctx))
	assert.Equal(t, StateStopped, supervisor.State())

	_, err = supervisor.Client()
	assert.ErrorIs(t, err, ErrPluginNotRunning)
	// Configuring a stopped plugin only stores the settings.
	assert.NoError(t, supervisor.Configure(ctx, map[string]any{"token": "later"}))

	assert.Equal(t, []State{StateStarting, StateRunning, StateStopped}, drainStates(states))
}

func TestSupervisorRejectsWrongCapabilities(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeWrongCaps, nil)

	err := supervisor.Start(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginHandshake)
	assert.Equal(t, StateError, supervisor.State())
	assert.ErrorIs(t, supervisor.LastError(), ErrPluginHandshake)
}

func TestSupervisorRejectsPipeOnAnotherMachine(t *testing.T) {
	// Dialing a pipe on another machine would hand it the credentials of
	// the host, so the handshake fails before anything dials it.
	supervisor := newTestSupervisor(t, pluginModeRemotePipe, nil)

	err := supervisor.Start(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginHandshake)
	assert.Contains(t, err.Error(), "not a named pipe on this machine")
}

func TestSupervisorRejectsIncompatibleAPIVersion(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeBadAPI, nil)

	err := supervisor.Start(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrIncompatibleAPIVersion)
	assert.Equal(t, StateError, supervisor.State())
}

func TestSupervisorRestartsAfterCrash(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeCrash, nil)
	// A high limit keeps the plugin in the restart loop for this test.
	supervisor.crashLimit = 100

	require.NoError(t, supervisor.Start(context.Background()))

	assert.Eventually(t, func() bool {
		return supervisor.Restarts() >= 2
	}, 10*time.Second, 10*time.Millisecond, "the crashed plugin was not restarted")
	assert.Error(t, supervisor.LastError())
}

func TestSupervisorStopsRestartingAfterThreeCrashes(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeCrash, nil)

	require.NoError(t, supervisor.Start(context.Background()))

	assert.Eventually(t, func() bool {
		return supervisor.State() == StateError
	}, 10*time.Second, 10*time.Millisecond, "the plugin never reached the error state")
	assert.Equal(t, 2, supervisor.Restarts())
	assert.Error(t, supervisor.LastError())

	// The restart loop is over, the state stays terminal.
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, StateError, supervisor.State())
	assert.Equal(t, 2, supervisor.Restarts())
}

func TestSupervisorKillsStubbornPlugin(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeStubborn, nil)
	supervisor.exitTimeout = 300 * time.Millisecond
	require.NoError(t, supervisor.Start(context.Background()))

	started := time.Now()
	require.NoError(t, supervisor.Stop(context.Background()))
	assert.Equal(t, StateStopped, supervisor.State())
	// Shutdown waits for the reply, then the exit grace period expires.
	assert.Less(t, time.Since(started), 5*time.Second)
}

func TestSupervisorKillsHungPlugin(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeDeaf, nil)
	supervisor.pingInterval = 50 * time.Millisecond
	supervisor.pingTimeout = 50 * time.Millisecond
	supervisor.pingFailures = 2
	supervisor.crashLimit = 100

	require.NoError(t, supervisor.Start(context.Background()))

	assert.Eventually(t, func() bool {
		return supervisor.Restarts() >= 1
	}, 10*time.Second, 10*time.Millisecond, "the hung plugin was not killed")
	assert.ErrorContains(t, supervisor.LastError(), "ping")
}

func TestSupervisorOnDemandStartsAndStopsIdle(t *testing.T) {
	supervisor := newTestSupervisor(t, pluginModeNormal, func(cfg *SupervisorConfig) {
		cfg.Lifecycle = protocol.LifecycleOnDemand
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	assert.Equal(t, StateStopped, supervisor.State())

	client, release, err := supervisor.Acquire(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateRunning, supervisor.State())

	var echo struct {
		Text string `json:"text"`
	}
	require.NoError(t, client.Call(ctx, "echo.hello", map[string]string{"text": "on demand"}, &echo))
	assert.Equal(t, "ON DEMAND", echo.Text)

	// While the acquisition is open the idle timer must not fire.
	time.Sleep(2 * supervisor.idleTimeout)
	assert.Equal(t, StateRunning, supervisor.State())

	release()
	assert.Eventually(t, func() bool {
		return supervisor.State() == StateStopped
	}, 10*time.Second, 10*time.Millisecond, "the idle plugin did not stop")

	// The next acquisition brings it back.
	_, release, err = supervisor.Acquire(ctx)
	require.NoError(t, err)
	defer release()
	assert.Equal(t, StateRunning, supervisor.State())
}

func TestSupervisorClientWithoutProcess(t *testing.T) {
	supervisor := NewSupervisor(SupervisorConfig{PluginID: "official.test"})

	_, err := supervisor.Client()
	assert.ErrorIs(t, err, ErrPluginNotRunning)

	_, release, err := supervisor.Acquire(context.Background())
	assert.ErrorIs(t, err, ErrPluginNotRunning)
	require.NotNil(t, release)

	assert.ErrorIs(t, supervisor.Start(context.Background()), ErrNoExecutableForPlatform)
	assert.NoError(t, supervisor.Stop(context.Background()))
}

// logsContain reports whether the stderr ring buffer holds a line.
func logsContain(supervisor *Supervisor, needle string) bool {
	for _, line := range supervisor.Logs() {
		if strings.Contains(line.Line, needle) {
			return true
		}
	}
	return false
}

// drainStates collects the transitions the supervisor reported so far.
func drainStates(states chan State) []State {
	seen := make([]State, 0, len(states))
	for {
		select {
		case state := <-states:
			seen = append(seen, state)
		default:
			return seen
		}
	}
}

func TestTimeoutCallerAppliesTheDefaultDeadline(t *testing.T) {
	serverIn, clientOut := io.Pipe()
	clientIn, serverOut := io.Pipe()
	server := jsonrpc.NewConn(serverIn, serverOut)
	client := jsonrpc.NewConn(clientIn, clientOut)

	release := make(chan struct{})
	server.Handle("hang", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-release
		return protocol.EmptyResult{}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = server.Serve(ctx) }()
	go func() { _ = client.Serve(ctx) }()
	t.Cleanup(func() {
		close(release)
		cancel()
		_ = server.Close()
		_ = client.Close()
	})

	caller := &timeoutCaller{conn: client, timeout: 50 * time.Millisecond}
	assert.ErrorIs(t, caller.Call(context.Background(), "hang", nil, nil), ErrCallTimeout)
	assert.NoError(t, caller.Notify(context.Background(), "hang", nil))
}

func TestLogRingKeepsTheTail(t *testing.T) {
	ring := newLogRing(3)
	for i := range 5 {
		ring.add(LogLine{Time: time.Now(), Line: fmt.Sprintf("line-%d", i)})
	}
	lines := ring.lines()
	require.Len(t, lines, 3)
	assert.Equal(t, "line-2", lines[0].Line)
	assert.Equal(t, "line-4", lines[2].Line)
}

func TestSameStringSet(t *testing.T) {
	assert.True(t, sameStringSet(nil, nil))
	assert.True(t, sameStringSet([]string{"a", "b"}, []string{"b", "a"}))
	assert.True(t, sameStringSet([]string{"a", "a"}, []string{"a"}))
	assert.False(t, sameStringSet([]string{"a"}, []string{"a", "b"}))
}
