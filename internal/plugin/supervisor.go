package plugin

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/uozi-tech/cosy/logger"
	"go.uber.org/zap"
)

// Supervisor timing defaults. Every one of them has an unexported field on
// Supervisor so tests can shrink it.
const (
	defaultHandshakeTimeout = 10 * time.Second
	defaultCallTimeout      = 30 * time.Second
	defaultPingInterval     = 15 * time.Second
	defaultPingTimeout      = 5 * time.Second
	defaultPingFailures     = 3
	defaultShutdownTimeout  = 5 * time.Second
	defaultExitTimeout      = 5 * time.Second
	defaultIdleTimeout      = 300 * time.Second
	defaultCrashWindow      = 5 * time.Minute
	defaultCrashLimit       = 3
	// stderrRingSize is how many plugin stderr lines are kept for the UI.
	stderrRingSize = 500
	// stderrMaxLineSize bounds one stderr line so a runaway plugin cannot
	// exhaust the host memory.
	stderrMaxLineSize = 1 << 20
)

// defaultCgroupRoot is where cgroup v2 is usually mounted.
const defaultCgroupRoot = "/sys/fs/cgroup"

// defaultBackoff is the delay before restart attempt n after a crash.
var defaultBackoff = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
}

// Environment variables injected into every plugin process. Credentials are
// never passed this way, plugins ask for them over the host API.
const (
	EnvPluginID         = "NGINX_UI_PLUGIN_ID"
	EnvPluginAPIVersion = "NGINX_UI_PLUGIN_API_VERSION"
	EnvPluginDataDir    = "NGINX_UI_PLUGIN_DATA_DIR"
	EnvHostVersion      = "NGINX_UI_VERSION"
	// EnvPluginHTTPSecret carries the per process secret a plugin serving the
	// http capability requires on every request, see httpSecretBytes.
	EnvPluginHTTPSecret = "NGINX_UI_PLUGIN_HTTP_SECRET"
)

// httpSecretBytes is the size of the random secret behind EnvPluginHTTPSecret.
const httpSecretBytes = 32

// State is the runtime state of one supervised plugin process.
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateError    State = "error"
)

// LogLine is one line the plugin wrote to stderr.
type LogLine struct {
	Time time.Time `json:"time"`
	Line string    `json:"line"`
}

// SupervisorConfig describes the one plugin process a Supervisor owns.
type SupervisorConfig struct {
	PluginID string
	// Dir is the plugin directory and becomes the working directory.
	Dir string
	// DataDir is the private writable directory handed to the plugin.
	DataDir  string
	Manifest *protocol.Manifest
	// Argv is the resolved command line, see ResolveExecutable.
	Argv        []string
	HostVersion string
	Locale      string
	Settings    map[string]any
	// Permissions are the ones the user approved for this plugin.
	Permissions []string
	// Lifecycle overrides the manifest lifecycle when it is not empty.
	Lifecycle string
	// Resources limits the process, see EffectiveResources. The zero value
	// runs it without limits.
	Resources ResourceLimits
	// CgroupRoot is the cgroup v2 mount point the limits are enforced
	// under. Empty means /sys/fs/cgroup.
	CgroupRoot string
	// HandshakeTimeout replaces the default when positive. Test binaries
	// built with the race detector need far longer than a real plugin.
	HandshakeTimeout time.Duration
	// IdleTimeout applies to on_demand plugins only.
	IdleTimeout time.Duration
	// OnStateChange is called outside the internal lock on every transition.
	OnStateChange func(State, error)
	// HostHandlers registers the host API on a fresh connection. It runs
	// before the handshake so a plugin can call back immediately.
	HostHandlers func(conn *jsonrpc.Conn)
	Logger       *zap.SugaredLogger
}

// process is one spawned plugin instance. A Supervisor owns at most one.
type process struct {
	cmd    *exec.Cmd
	conn   *jsonrpc.Conn
	stdin  *os.File
	stdout *os.File
	// done is closed once the process was reaped and the exit was handled.
	done chan struct{}
	// hung records that the ping loop gave up on this process.
	hung       atomic.Bool
	initResult protocol.InitializeResult
	// httpSecret is the secret this process was started with, empty when it
	// does not serve the http capability on a listener.
	httpSecret string
	// grpc carries capability calls when the plugin serves gRPC. It is set
	// before the process is published.
	grpc *grpcRoute
	// cgroup confines the process, nil when it runs without limits.
	cgroup *pluginCgroup
}

// Supervisor runs one plugin process, keeps it alive and exposes a client for
// capability calls.
type Supervisor struct {
	cfg SupervisorConfig
	log *zap.SugaredLogger

	// opMu serialises the lifecycle operations so two spawns cannot overlap.
	opMu sync.Mutex

	mu           sync.Mutex
	state        State
	lastErr      error
	proc         *process
	restarts     int
	crashes      []time.Time
	stopping     bool
	acquisitions int
	restartTimer *time.Timer
	idleTimer    *time.Timer

	ring *logRing

	// Test hooks. They are set before Start and read afterwards.
	handshakeTimeout time.Duration
	callTimeout      time.Duration
	pingInterval     time.Duration
	pingTimeout      time.Duration
	pingFailures     int
	shutdownTimeout  time.Duration
	exitTimeout      time.Duration
	idleTimeout      time.Duration
	crashWindow      time.Duration
	crashLimit       int
	backoff          []time.Duration
	extraEnv         []string
	grpcDialTimeout  time.Duration
}

// NewSupervisor prepares a supervisor. Nothing is spawned until Start runs.
func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	log := cfg.Logger
	if log == nil {
		log = logger.GetLogger()
	}
	idle := cfg.IdleTimeout
	if idle <= 0 {
		if cfg.Manifest != nil && cfg.Manifest.Server != nil && cfg.Manifest.Server.IdleTimeoutSeconds > 0 {
			idle = time.Duration(cfg.Manifest.Server.IdleTimeoutSeconds) * time.Second
		} else {
			idle = defaultIdleTimeout
		}
	}
	handshakeTimeout := defaultHandshakeTimeout
	if cfg.HandshakeTimeout > 0 {
		handshakeTimeout = cfg.HandshakeTimeout
	}
	return &Supervisor{
		cfg:              cfg,
		log:              log,
		state:            StateStopped,
		ring:             newLogRing(stderrRingSize),
		handshakeTimeout: handshakeTimeout,
		callTimeout:      defaultCallTimeout,
		pingInterval:     defaultPingInterval,
		pingTimeout:      defaultPingTimeout,
		pingFailures:     defaultPingFailures,
		shutdownTimeout:  defaultShutdownTimeout,
		exitTimeout:      defaultExitTimeout,
		idleTimeout:      idle,
		crashWindow:      defaultCrashWindow,
		crashLimit:       defaultCrashLimit,
		backoff:          slices.Clone(defaultBackoff),
		grpcDialTimeout:  defaultGRPCDialTimeout,
	}
}

// State returns the current runtime state.
func (s *Supervisor) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// LastError returns the failure that produced the current state, if any.
func (s *Supervisor) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// Restarts counts the automatic restarts since the last manual Start.
func (s *Supervisor) Restarts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restarts
}

// Logs returns the buffered stderr lines, oldest first.
func (s *Supervisor) Logs() []LogLine {
	return s.ring.lines()
}

// InitializeResult returns what the running plugin answered to the handshake.
func (s *Supervisor) InitializeResult() (protocol.InitializeResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil {
		return protocol.InitializeResult{}, false
	}
	return s.proc.initResult, true
}

// HTTPSecret returns the secret the running process expects on every http
// request, and false when there is no process or it has no such listener.
func (s *Supervisor) HTTPSecret() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil || s.proc.httpSecret == "" {
		return "", false
	}
	return s.proc.httpSecret, true
}

// newHTTPSecret returns a fresh secret for a plugin that serves the http
// capability on a listener of its own, and an empty string for any other.
func (s *Supervisor) newHTTPSecret() (string, error) {
	m := s.cfg.Manifest
	if m == nil || m.HTTP == nil || m.HTTP.Listen != "unix" || !slices.Contains(m.Capabilities, protocol.CapabilityHTTP) {
		return "", nil
	}
	raw := make([]byte, httpSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate the http secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Resources reports the limits of the plugin process and whether the running
// process is confined to them.
func (s *Supervisor) Resources() ResourceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	enforced := s.proc != nil && s.proc.cgroup != nil
	return resourceStatus(s.cfg.Resources, enforced)
}

// cgroupRoot is the configured cgroup root or the default one.
func (s *Supervisor) cgroupRoot() string {
	if s.cfg.CgroupRoot != "" {
		return s.cfg.CgroupRoot
	}
	return defaultCgroupRoot
}

// Lifecycle reports the effective lifecycle of the plugin.
func (s *Supervisor) Lifecycle() string {
	if s.cfg.Lifecycle != "" {
		return s.cfg.Lifecycle
	}
	if s.cfg.Manifest != nil && s.cfg.Manifest.Server != nil && s.cfg.Manifest.Server.Lifecycle != "" {
		return s.cfg.Manifest.Server.Lifecycle
	}
	return protocol.LifecycleResident
}

// Start spawns the process and completes the handshake. Starting an already
// running plugin is a no-op, a manual start clears the crash bookkeeping.
func (s *Supervisor) Start(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if s.proc != nil {
		s.mu.Unlock()
		return nil
	}
	s.stopping = false
	s.restarts = 0
	s.crashes = nil
	s.lastErr = nil
	s.stopTimersLocked()
	s.state = StateStarting
	s.mu.Unlock()
	s.notify(StateStarting, nil)

	if err := s.spawn(ctx); err != nil {
		s.mu.Lock()
		s.state = StateError
		s.lastErr = err
		s.mu.Unlock()
		s.notify(StateError, err)
		return err
	}
	return nil
}

// Stop asks the plugin to exit and kills it when it does not.
func (s *Supervisor) Stop(ctx context.Context) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	s.stopping = true
	s.stopTimersLocked()
	p := s.proc
	if p == nil {
		changed := s.state != StateStopped
		s.state = StateStopped
		s.mu.Unlock()
		if changed {
			s.notify(StateStopped, nil)
		}
		return nil
	}
	s.mu.Unlock()

	// The plugin closes its gRPC listener while it stops, which is expected.
	p.grpc.markStopping()
	shutdownCtx, cancel := context.WithTimeout(ctx, s.shutdownTimeout)
	if err := p.conn.Call(shutdownCtx, protocol.MethodShutdown, nil, nil); err != nil {
		s.log.Debugf("[plugin:%s] shutdown request failed: %v", s.cfg.PluginID, err)
	}
	cancel()
	_ = p.conn.Notify(context.Background(), protocol.MethodExit, nil)

	timer := time.NewTimer(s.exitTimeout)
	defer timer.Stop()
	select {
	case <-p.done:
	case <-timer.C:
		s.log.Warnf("[plugin:%s] did not exit in time, killing it", s.cfg.PluginID)
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		<-p.done
	}
	_ = p.conn.Close()
	return nil
}

// Configure pushes new settings into a running plugin. The settings are kept
// for the next start even when the plugin is down.
func (s *Supervisor) Configure(ctx context.Context, settings map[string]any) error {
	s.mu.Lock()
	s.cfg.Settings = settings
	p := s.proc
	running := p != nil && s.state == StateRunning
	s.mu.Unlock()
	if !running {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, s.callTimeout)
	defer cancel()
	return WrapRPCError(p.conn.Call(callCtx, protocol.MethodConfigure, protocol.ConfigureParams{Settings: settings}, nil))
}

// Client returns the caller for a running plugin.
func (s *Supervisor) Client() (jsonrpc.Caller, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil || s.state != StateRunning {
		return nil, ErrPluginNotRunning
	}
	return &timeoutCaller{conn: s.proc.conn, timeout: s.callTimeout, route: s.proc.grpc}, nil
}

// Acquire returns a caller and the matching release function. An on_demand
// plugin is started on the first acquisition and stops again once the last
// release has been idle for IdleTimeout.
func (s *Supervisor) Acquire(ctx context.Context) (jsonrpc.Caller, func(), error) {
	noop := func() {}
	if s.Lifecycle() != protocol.LifecycleOnDemand {
		client, err := s.Client()
		if err != nil {
			return nil, noop, err
		}
		return client, noop, nil
	}

	s.mu.Lock()
	s.acquisitions++
	s.stopIdleTimerLocked()
	running := s.proc != nil && s.state == StateRunning
	s.mu.Unlock()

	var once sync.Once
	release := func() { once.Do(s.release) }

	if !running {
		if err := s.Start(ctx); err != nil {
			release()
			return nil, noop, err
		}
	}
	client, err := s.Client()
	if err != nil {
		release()
		return nil, noop, err
	}
	return client, release, nil
}

func (s *Supervisor) release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acquisitions > 0 {
		s.acquisitions--
	}
	if s.acquisitions == 0 && s.proc != nil {
		s.armIdleTimerLocked()
	}
}

// spawn starts the process, wires the connection and performs the handshake.
func (s *Supervisor) spawn(ctx context.Context) error {
	if len(s.cfg.Argv) == 0 {
		return ErrNoExecutableForPlatform
	}

	cmd := exec.Command(s.cfg.Argv[0], s.cfg.Argv[1:]...)
	cmd.Dir = s.cfg.Dir
	httpSecret, err := s.newHTTPSecret()
	if err != nil {
		return err
	}
	cmd.Env = s.env(httpSecret)

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return err
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		closeAll(stdinR, stdinW)
		return err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		closeAll(stdinR, stdinW, stdoutR, stdoutW)
		return err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdinR, stdoutW, stderrW

	cmd, cg, err := s.startProcess(cmd)
	if err != nil {
		closeAll(stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW)
		return err
	}
	// The child owns its ends now, the parent keeps only the other half.
	closeAll(stdinR, stdoutW, stderrW)

	p := &process{cmd: cmd, stdin: stdinW, stdout: stdoutR, done: make(chan struct{}), cgroup: cg, httpSecret: httpSecret}
	p.conn = jsonrpc.NewConn(stdoutR, stdinW, jsonrpc.WithLogger(s.log))
	if s.cfg.HostHandlers != nil {
		s.cfg.HostHandlers(p.conn)
	}
	go func() { _ = p.conn.Serve(context.Background()) }()
	go s.readStderr(stderrR)

	if err = s.handshake(ctx, p); err != nil {
		_ = p.conn.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		_ = p.cmd.Wait()
		s.releaseCgroup(p)
		return err
	}
	s.connectGRPC(ctx, p)

	s.mu.Lock()
	s.proc = p
	s.state = StateRunning
	s.lastErr = nil
	onDemandIdle := s.acquisitions == 0 && s.Lifecycle() == protocol.LifecycleOnDemand
	if onDemandIdle {
		s.armIdleTimerLocked()
	}
	s.mu.Unlock()
	s.notify(StateRunning, nil)

	go s.waitProcess(p)
	go s.pingLoop(p)
	return nil
}

// handshake runs plugin.initialize and verifies the answer against the
// manifest, then confirms with plugin.initialized.
func (s *Supervisor) handshake(ctx context.Context, p *process) error {
	handshakeCtx, cancel := context.WithTimeout(ctx, s.handshakeTimeout)
	defer cancel()

	// Configure may replace the settings at any time, so take a snapshot.
	s.mu.Lock()
	settings := s.cfg.Settings
	s.mu.Unlock()

	params := protocol.InitializeParams{
		Host: protocol.HostInfo{
			Version: s.cfg.HostVersion,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Locale:  s.cfg.Locale,
		},
		Settings:    settings,
		Permissions: s.cfg.Permissions,
	}
	var result protocol.InitializeResult
	if err := p.conn.Call(handshakeCtx, protocol.MethodInitialize, params, &result); err != nil {
		return fmt.Errorf("%w: %v", ErrPluginHandshake, err)
	}
	if result.APIVersion != protocol.APIVersion {
		return fmt.Errorf("%w: plugin speaks api version %d, host speaks %d",
			ErrIncompatibleAPIVersion, result.APIVersion, protocol.APIVersion)
	}
	var declared []string
	if s.cfg.Manifest != nil {
		declared = s.cfg.Manifest.Capabilities
	}
	if !sameStringSet(result.Capabilities, declared) {
		return fmt.Errorf("%w: plugin reports capabilities %v, the manifest declares %v",
			ErrPluginHandshake, result.Capabilities, declared)
	}
	p.initResult = result

	if err := p.conn.Notify(handshakeCtx, protocol.MethodInitialized, nil); err != nil {
		return fmt.Errorf("%w: %v", ErrPluginHandshake, err)
	}
	return nil
}

// waitProcess reaps the process and hands the exit to the state machine.
func (s *Supervisor) waitProcess(p *process) {
	waitErr := p.cmd.Wait()
	_ = p.conn.Close()
	p.grpc.close()
	s.releaseCgroup(p)
	s.handleExit(p, waitErr)
	close(p.done)
}

// releaseCgroup removes the group of a process that exited.
func (s *Supervisor) releaseCgroup(p *process) {
	if p.cgroup == nil {
		return
	}
	if err := p.cgroup.remove(); err != nil {
		s.log.Debugf("[plugin:%s] remove cgroup %s: %v", s.cfg.PluginID, p.cgroup.dir, err)
	}
}

// handleExit decides what a finished process means: an expected stop, a
// backoff restart or the terminal error state.
func (s *Supervisor) handleExit(p *process, waitErr error) {
	s.mu.Lock()
	if s.proc != p {
		// A newer process already took over, nothing to do.
		s.mu.Unlock()
		return
	}
	s.proc = nil
	s.stopIdleTimerLocked()

	if s.stopping {
		s.state = StateStopped
		s.mu.Unlock()
		s.notify(StateStopped, nil)
		return
	}

	cause := exitCause(p, waitErr)
	state := s.recordFailureLocked(cause)
	s.mu.Unlock()

	s.log.Warnf("[plugin:%s] process exited: %v", s.cfg.PluginID, cause)
	s.notify(state, cause)
}

// recordFailureLocked books one crash and picks the next state. The caller
// must hold s.mu.
func (s *Supervisor) recordFailureLocked(cause error) State {
	now := time.Now()
	s.crashes = append(s.crashes, now)
	s.crashes = slices.DeleteFunc(s.crashes, func(at time.Time) bool {
		return now.Sub(at) > s.crashWindow
	})
	s.lastErr = cause

	switch {
	case len(s.crashes) >= s.crashLimit:
		s.state = StateError
	case s.Lifecycle() == protocol.LifecycleOnDemand && s.acquisitions == 0:
		// Nobody is waiting for it, the next Acquire starts it again.
		s.state = StateStopped
	default:
		delay := s.backoff[min(s.restarts, len(s.backoff)-1)]
		s.restarts++
		s.restartTimer = time.AfterFunc(delay, s.restart)
		s.state = StateStarting
	}
	return s.state
}

// restart is the backoff timer callback.
func (s *Supervisor) restart() {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if s.stopping || s.proc != nil {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	if err := s.spawn(context.Background()); err != nil {
		s.log.Warnf("[plugin:%s] restart failed: %v", s.cfg.PluginID, err)
		s.mu.Lock()
		state := s.recordFailureLocked(err)
		s.mu.Unlock()
		s.notify(state, err)
	}
}

// pingLoop keeps checking that the plugin still answers and kills it when it
// misses too many pings in a row.
func (s *Supervisor) pingLoop(p *process) {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-p.done:
			return
		case <-p.conn.Done():
			return
		case <-ticker.C:
		}

		pingCtx, cancel := context.WithTimeout(context.Background(), s.pingTimeout)
		err := p.conn.Call(pingCtx, protocol.MethodPing, nil, nil)
		cancel()
		switch {
		case err == nil:
			failures = 0
		case errors.Is(err, jsonrpc.ErrClosed):
			return
		default:
			failures++
			s.log.Warnf("[plugin:%s] ping failed (%d/%d): %v", s.cfg.PluginID, failures, s.pingFailures, err)
			if failures >= s.pingFailures {
				p.hung.Store(true)
				if p.cmd.Process != nil {
					_ = p.cmd.Process.Kill()
				}
				return
			}
		}
	}
}

// readStderr forwards the plugin log to the host logger and keeps the tail for
// the UI.
func (s *Supervisor) readStderr(r *os.File) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64<<10), stderrMaxLineSize)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		s.ring.add(LogLine{Time: time.Now(), Line: line})
		s.log.Infof("[plugin:%s] %s", s.cfg.PluginID, line)
	}
}

// hostOnlyEnv are variables the host sets for its own ACME client. They must
// not be inherited: LEGO_DISABLE_CNAME_SUPPORT stops the core from following
// the challenge record CNAME, while a dns01 plugin is expected to follow it
// unless the certificate asked otherwise.
var hostOnlyEnv = []string{
	"LEGO_DISABLE_CNAME_SUPPORT",
}

// proxyEnv are the variables standard HTTP clients read to find a proxy, in
// both spellings.
var proxyEnv = []string{
	"HTTP_PROXY", "http_proxy",
	"HTTPS_PROXY", "https_proxy",
	"NO_PROXY", "no_proxy",
}

// defaultNoProxy keeps traffic to the machine itself off the proxy.
const defaultNoProxy = "localhost,127.0.0.1,::1"

// hostHTTPProxy returns the outbound proxy of the host configuration, empty
// when none is set. It is read on every spawn, so a restart picks up a change.
var hostHTTPProxy = func() string {
	return strings.TrimSpace(settings.HTTPSettings.HTTPProxy)
}

// env builds the child environment. Credentials never travel this way: the
// host configuration variables, which carry the node secret among others,
// are dropped along with hostOnlyEnv. The proxy variables follow the network
// permission: without it the plugin gets none, not even an inherited one, and
// with it the proxy of the host configuration, when one is set, replaces them.
func (s *Supervisor) env(httpSecret string) []string {
	parent := os.Environ()
	hasNetwork := slices.Contains(s.cfg.Permissions, protocol.PermissionNetwork)
	proxy := ""
	if hasNetwork {
		proxy = hostHTTPProxy()
	}
	dropProxy := !hasNetwork || proxy != ""

	env := make([]string, 0, len(parent)+4+len(proxyEnv)+len(s.extraEnv))
	noProxy := defaultNoProxy
	for _, entry := range parent {
		if isHostOnlyEnv(entry) {
			continue
		}
		if dropProxy && isProxyEnv(entry) {
			if key, value, _ := strings.Cut(entry, "="); proxy != "" && value != "" && strings.EqualFold(key, "NO_PROXY") {
				noProxy = mergeNoProxy(noProxy, value)
			}
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		EnvPluginID+"="+s.cfg.PluginID,
		EnvPluginAPIVersion+"="+strconv.Itoa(protocol.APIVersion),
		EnvPluginDataDir+"="+s.cfg.DataDir,
		EnvHostVersion+"="+s.cfg.HostVersion,
	)
	if httpSecret != "" {
		env = append(env, EnvPluginHTTPSecret+"="+httpSecret)
	}
	if proxy != "" {
		env = append(env,
			"HTTP_PROXY="+proxy, "http_proxy="+proxy,
			"HTTPS_PROXY="+proxy, "https_proxy="+proxy,
			"NO_PROXY="+noProxy, "no_proxy="+noProxy,
		)
	}
	return append(env, s.extraEnv...)
}

func isProxyEnv(entry string) bool {
	key, _, ok := strings.Cut(entry, "=")
	return ok && slices.Contains(proxyEnv, key)
}

// mergeNoProxy appends the entries of extra that base does not list yet.
func mergeNoProxy(base, extra string) string {
	seen := map[string]bool{}
	for _, item := range strings.Split(base, ",") {
		seen[strings.TrimSpace(item)] = true
	}
	merged := base
	for _, item := range strings.Split(extra, ",") {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		merged += "," + item
	}
	return merged
}

func isHostOnlyEnv(entry string) bool {
	key, _, ok := strings.Cut(entry, "=")
	if !ok {
		return false
	}
	return strings.HasPrefix(key, settings.EnvPrefix) || slices.Contains(hostOnlyEnv, key)
}

func (s *Supervisor) notify(state State, err error) {
	if s.cfg.OnStateChange != nil {
		s.cfg.OnStateChange(state, err)
	}
}

func (s *Supervisor) stopTimersLocked() {
	if s.restartTimer != nil {
		s.restartTimer.Stop()
		s.restartTimer = nil
	}
	s.stopIdleTimerLocked()
}

func (s *Supervisor) stopIdleTimerLocked() {
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
}

func (s *Supervisor) armIdleTimerLocked() {
	s.stopIdleTimerLocked()
	s.idleTimer = time.AfterFunc(s.idleTimeout, s.idleStop)
}

// idleStop stops an on_demand plugin that nobody acquired for a while.
func (s *Supervisor) idleStop() {
	s.mu.Lock()
	idle := s.acquisitions == 0 && s.proc != nil
	s.mu.Unlock()
	if !idle {
		return
	}
	s.log.Debugf("[plugin:%s] idle timeout reached, stopping", s.cfg.PluginID)
	if err := s.Stop(context.Background()); err != nil {
		s.log.Warnf("[plugin:%s] idle stop failed: %v", s.cfg.PluginID, err)
	}
}

// exitCause turns a process exit into the error reported to the manager.
func exitCause(p *process, waitErr error) error {
	if p.hung.Load() {
		return errors.New("plugin stopped answering pings")
	}
	if waitErr != nil {
		return waitErr
	}
	return errors.New("plugin exited unexpectedly")
}

// logRing keeps the last n stderr lines of a plugin.
type logRing struct {
	mu   sync.Mutex
	size int
	buf  []LogLine
}

func newLogRing(size int) *logRing {
	return &logRing{size: size}
}

func (r *logRing) add(line LogLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, line)
	if len(r.buf) > r.size {
		r.buf = slices.Delete(r.buf, 0, len(r.buf)-r.size)
	}
}

func (r *logRing) lines() []LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.buf)
}

// sameStringSet compares two lists as sets, ignoring order and duplicates.
func sameStringSet(a, b []string) bool {
	left := slices.Clone(a)
	right := slices.Clone(b)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(slices.Compact(left), slices.Compact(right))
}

func closeAll(files ...io.Closer) {
	for _, f := range files {
		if f != nil {
			_ = f.Close()
		}
	}
}
