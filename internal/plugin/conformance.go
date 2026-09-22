package plugin

// This file drives a real plugin process through the numbered conformance
// cases in nginx-ui-plugin-spec/spec/09-conformance.md, for
// "nginx-ui plugin conformance". It reuses the production Supervisor and
// RegisterHostHandlers exactly as the plugin manager does, so a pass here is
// evidence the plugin works against the real host, not a reimplementation of
// the wire protocol.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// CaseStatus is the outcome of one conformance case.
type CaseStatus string

const (
	StatusPass CaseStatus = "pass"
	StatusFail CaseStatus = "fail"
	StatusSkip CaseStatus = "skip"
	StatusWarn CaseStatus = "warn"
)

// CaseResult is the outcome of one numbered requirement Conformance checked.
type CaseResult struct {
	Rule string
	Name string
	// Transport is "stdio" or "grpc" for a case that ran over that
	// transport, empty for one that does not depend on it.
	Transport string
	Status    CaseStatus
	Message   string
	Duration  time.Duration
}

// Values of ConformanceOptions.Transport.
const (
	TransportFlagStdio = "stdio"
	TransportFlagGRPC  = "grpc"
	TransportFlagBoth  = "both"
)

// ConformanceOptions configures Conformance.
type ConformanceOptions struct {
	// Capabilities restricts which capability specific cases run, beyond the
	// core lifecycle and wire protocol cases which always run. Empty runs
	// every capability the manifest declares.
	Capabilities []string
	// Timeout bounds the whole run, including starting and stopping the
	// plugin process. Zero uses defaultConformanceTimeout.
	Timeout time.Duration
	// Transport selects which transports the protocol and capability cases
	// run over: TransportFlagStdio, TransportFlagGRPC or TransportFlagBoth.
	// Empty runs both when the plugin advertises grpc, stdio otherwise.
	Transport string
}

// ConformanceReport lists every case Conformance checked, in the order they
// ran.
type ConformanceReport struct {
	Cases []CaseResult
}

// Passed reports whether every case passed, was skipped, or was a warning.
func (r *ConformanceReport) Passed() bool {
	for _, c := range r.Cases {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

// defaultConformanceTimeout bounds a run when the caller does not set one.
const defaultConformanceTimeout = 90 * time.Second

// maxWebappBundleSize is the WEB-1 budget for a plugin's browser bundle.
const maxWebappBundleSize = 2 << 20 // 2 MiB

// recorder appends one CaseResult to the report being built.
type recorder func(rule, name string, status CaseStatus, dur time.Duration, format string, args ...any)

// Conformance extracts or loads the plugin at path, runs it under a real
// Supervisor, and exercises it against the numbered conformance cases. It
// returns a non-nil error only when the plugin could not even be prepared
// (bad path, invalid manifest, no executable for this platform); a plugin
// that misbehaves once running is reported as failed cases, not an error.
func Conformance(ctx context.Context, path string, opts ConformanceOptions) (*ConformanceReport, error) {
	report := &ConformanceReport{}

	switch opts.Transport {
	case "", TransportFlagStdio, TransportFlagGRPC, TransportFlagBoth:
	default:
		return nil, fmt.Errorf("unknown transport %q, want stdio, grpc or both", opts.Transport)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultConformanceTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dir, cleanup, manifest, err := loadPluginForConformance(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	argv, err := ResolveExecutable(manifest, dir)
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}

	dataDir, err := os.MkdirTemp("", "nginx-ui-plugin-conformance-data-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dataDir)

	backend := &conformanceBackend{}
	sup := NewSupervisor(SupervisorConfig{
		PluginID:    manifest.ID,
		Dir:         dir,
		DataDir:     dataDir,
		Manifest:    manifest,
		Argv:        argv,
		HostVersion: "0.0.0-conformance",
		Locale:      "en",
		Settings:    map[string]any{},
		Permissions: manifest.Permissions,
		Lifecycle:   protocol.LifecycleOnDemand,
		IdleTimeout: timeout,
		HostHandlers: func(conn *jsonrpc.Conn) {
			RegisterHostHandlers(conn, manifest.ID, manifest.Permissions, backend)
		},
	})

	recordOn := func(transport string) recorder {
		return func(rule, name string, status CaseStatus, dur time.Duration, format string, args ...any) {
			report.Cases = append(report.Cases, CaseResult{
				Rule: rule, Name: name, Transport: transport, Status: status, Duration: dur,
				Message: fmt.Sprintf(format, args...),
			})
		}
	}
	record := recordOn("")

	started := time.Now()
	_, release, err := sup.Acquire(runCtx)
	handshakeElapsed := time.Since(started)
	if err != nil {
		record("LIFE-1", "handshake", StatusFail, handshakeElapsed, "plugin.initialize failed: %v", err)
		checkWebapp(dir, manifest, record)
		return report, nil
	}
	defer release()

	init, _ := sup.InitializeResult()
	if init.APIVersion != protocol.APIVersion {
		record("LIFE-3", "handshake api_version", StatusFail, handshakeElapsed,
			"plugin reported api_version %d, host speaks %d", init.APIVersion, protocol.APIVersion)
	} else {
		record("LIFE-1", "handshake", StatusPass, handshakeElapsed,
			"completed in %s with api_version %d", handshakeElapsed.Round(time.Millisecond), init.APIVersion)
	}

	if sameStringSet(init.Capabilities, manifest.Capabilities) {
		record("LIFE-4", "capabilities match manifest", StatusPass, 0, "capabilities: %v", init.Capabilities)
	} else {
		record("LIFE-4", "capabilities match manifest", StatusFail, 0,
			"handshake reported %v, manifest declares %v", init.Capabilities, manifest.Capabilities)
	}

	targets := conformanceTargetsOf(manifest, effectiveCapabilities(manifest, opts))

	advertised := slices.Contains(init.Transports, protocol.TransportGRPC)
	runStdio := opts.Transport != TransportFlagGRPC
	runGRPC := opts.Transport == TransportFlagGRPC || opts.Transport == TransportFlagBoth ||
		(opts.Transport == "" && advertised)

	var stdioCaller jsonrpc.Caller
	if runStdio {
		stdioCaller, err = sup.stdioClient()
		if err != nil {
			record("LIFE-1", "handshake", StatusFail, 0, "the plugin stopped right after the handshake: %v", err)
			return report, nil
		}
		runStdioCases(runCtx, sup, stdioCaller, targets, recordOn(protocol.TransportStdio))
	}

	var grpcClient *grpcbridge.Client
	if runGRPC {
		grpcRecord := recordOn(protocol.TransportGRPC)
		grpcClient = connectConformanceGRPC(runCtx, sup, init, dataDir, grpcRecord)
		if grpcClient != nil {
			runGRPCCases(runCtx, grpcClient, targets, grpcRecord)
		}
	}

	if stdioCaller != nil && grpcClient != nil {
		runTransportParity(runCtx, stdioCaller, &grpcConformanceCaller{client: grpcClient}, targets, record)
	}
	if grpcClient != nil {
		_ = grpcClient.Close()
	}

	checkWebapp(dir, manifest, record)

	stopStarted := time.Now()
	stopErr := sup.Stop(context.Background())
	stopElapsed := time.Since(stopStarted)
	switch {
	case stopErr != nil:
		record("LIFE-10", "shutdown and exit", StatusFail, stopElapsed, "Stop failed: %v", stopErr)
	case sup.State() != StateStopped:
		record("LIFE-10", "shutdown and exit", StatusFail, stopElapsed, "state is %s after Stop", sup.State())
	default:
		record("LIFE-10", "shutdown and exit", StatusPass, stopElapsed, "stopped in %s", stopElapsed.Round(time.Millisecond))
	}

	return report, nil
}

// conformanceTargets is what the capability cases run against: the first
// entry of every capability block the run covers. Empty fields skip the cases
// of that capability.
type conformanceTargets struct {
	dns01Code string
	notify    *protocol.NotifyChannel
	probeCode string
	// mcpUnknownTool is a tool name the manifest does not declare, empty when
	// the mcp cases do not run.
	mcpUnknownTool string
}

// conformanceTargetsOf picks the targets of the capabilities being tested.
func conformanceTargetsOf(manifest *protocol.Manifest, capabilities []string) conformanceTargets {
	var targets conformanceTargets
	if slices.Contains(capabilities, protocol.CapabilityDNS01) && manifest.DNS01 != nil && len(manifest.DNS01.Providers) > 0 {
		targets.dns01Code = manifest.DNS01.Providers[0].Code
	}
	if slices.Contains(capabilities, protocol.CapabilityNotify) && manifest.Notify != nil && len(manifest.Notify.Channels) > 0 {
		channel := manifest.Notify.Channels[0]
		targets.notify = &channel
	}
	if slices.Contains(capabilities, protocol.CapabilityProbe) && manifest.Probe != nil && len(manifest.Probe.Kinds) > 0 {
		targets.probeCode = manifest.Probe.Kinds[0].Code
	}
	if slices.Contains(capabilities, protocol.CapabilityMCP) {
		targets.mcpUnknownTool = unknownMCPTool(manifest)
	}
	return targets
}

// unknownMCPTool returns a tool name the manifest does not declare.
func unknownMCPTool(manifest *protocol.Manifest) string {
	declared := map[string]bool{}
	if manifest.MCP != nil {
		for _, tool := range manifest.MCP.Tools {
			declared[tool.Name] = true
		}
	}
	name := "nginx-ui-conformance-unknown-tool"
	for declared[name] {
		name += "-x"
	}
	return name
}

// runCapabilityCases runs the capability cases shared by both transports.
func runCapabilityCases(ctx context.Context, caller jsonrpc.Caller, targets conformanceTargets, record recorder) {
	if targets.dns01Code != "" {
		runDNS01Cases(ctx, caller, targets.dns01Code, record)
	}
	if targets.notify != nil {
		runNotifyValidate(ctx, caller, *targets.notify, record)
	}
	if targets.probeCode != "" {
		runProbeCheck(ctx, caller, targets.probeCode, record)
	}
	if targets.mcpUnknownTool != "" {
		runMCPUnknownTool(ctx, caller, targets.mcpUnknownTool, record)
	}
}

// runStdioCases runs the protocol and capability cases over stdio.
func runStdioCases(ctx context.Context, sup *Supervisor, caller jsonrpc.Caller, targets conformanceTargets, record recorder) {
	runPing(ctx, caller, record)
	runUnknownMethod(ctx, caller, record)
	runNotificationThenPing(ctx, caller, record)
	runConcurrentPings(ctx, caller, record)
	runStdoutHygiene(sup, record)

	if targets.dns01Code != "" {
		runInvalidParams(ctx, caller, record)
	}
	runCapabilityCases(ctx, caller, targets, record)
}

// connectConformanceGRPC checks WIRE-11: the plugin advertises grpc and its
// endpoint answers plugin.ping. It dials a client of its own, so the gRPC
// cases fail instead of silently falling back to stdio.
func connectConformanceGRPC(ctx context.Context, sup *Supervisor, init protocol.InitializeResult, dataDir string, record recorder) *grpcbridge.Client {
	endpoint, ok := grpcbridge.EndpointFor(init, dataDir)
	if !ok {
		record("WIRE-11", "grpc transport", StatusFail, 0,
			"the plugin does not list grpc in transports (got %v)", init.Transports)
		return nil
	}

	dialCtx, cancel := context.WithTimeout(ctx, defaultGRPCDialTimeout)
	defer cancel()
	started := time.Now()
	client, err := grpcbridge.Dial(dialCtx, endpoint)
	if err == nil {
		if err = client.Call(dialCtx, protocol.MethodPing, nil, nil); err != nil {
			_ = client.Close()
		}
	}
	elapsed := time.Since(started)
	if err != nil {
		record("WIRE-11", "grpc transport", StatusFail, elapsed, "cannot use the advertised endpoint %s: %v", endpoint, err)
		return nil
	}

	record("WIRE-11", "grpc transport", StatusPass, elapsed,
		"%s answered plugin.ping; the host routes capability calls over %s", endpoint, sup.Transport())
	return client
}

// runGRPCCases runs the protocol and capability cases over gRPC. The
// notification and stdout cases are stdio only.
func runGRPCCases(ctx context.Context, client *grpcbridge.Client, targets conformanceTargets, record recorder) {
	caller := &grpcConformanceCaller{client: client}
	runPing(ctx, caller, record)
	runUnknownMethod(ctx, caller, record)
	runConcurrentPings(ctx, caller, record)

	if targets.dns01Code != "" {
		runGRPCInvalidParams(ctx, client, record)
	}
	runCapabilityCases(ctx, caller, targets, record)
}

// unknownGRPCMethod is a gRPC path outside the contract.
const unknownGRPCMethod = "/nginxui.plugin.v1.Conformance/DoesNotExist"

// grpcConformanceCaller sends every call over gRPC, including the lifecycle
// ones the production router keeps on stdio. A method outside the contract
// has no gRPC path, so it is sent to unknownGRPCMethod and the plugin answers
// for itself.
type grpcConformanceCaller struct {
	client *grpcbridge.Client
}

func (c *grpcConformanceCaller) Call(ctx context.Context, method string, params any, result any) error {
	if _, ok := grpcbridge.Lookup(method); ok {
		return c.client.Call(ctx, method, params, result)
	}
	_, err := c.client.Invoke(ctx, unknownGRPCMethod, nil)
	if err == nil {
		return fmt.Errorf("%s answered without an error", unknownGRPCMethod)
	}
	return grpcbridge.FromStatus(ctx, err, false)
}

func (c *grpcConformanceCaller) Notify(context.Context, string, any) error {
	return errors.New("notifications travel on stdio")
}

// runGRPCInvalidParams checks WIRE-6 over gRPC: request bytes that are no
// valid message answer INVALID_ARGUMENT or INTERNAL instead of hanging.
func runGRPCInvalidParams(ctx context.Context, client *grpcbridge.Client, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	m, _ := grpcbridge.Lookup(protocol.MethodDNS01Options)
	_, err := client.Invoke(callCtx, m.FullMethod, []byte{0xff, 0xff, 0xff})
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		record("WIRE-6", "invalid params", StatusFail, 0, "the call hung instead of answering")
		return
	}
	if err == nil {
		record("WIRE-6", "invalid params", StatusFail, 0, "a malformed request message was accepted")
		return
	}
	err = grpcbridge.FromStatus(callCtx, err, true)
	if perr, ok := jsonrpc.AsProtocolError(err); ok && (perr.Code == protocol.CodeInvalidParams || perr.Code == protocol.CodeInternalError) {
		record("WIRE-6", "invalid params", StatusPass, 0, "a malformed request message answered with code %d", perr.Code)
		return
	}
	if isOptionalUnimplemented(err) {
		record("WIRE-6", "invalid params", StatusSkip, 0, "dns01.options is not implemented, cannot exercise this case")
		return
	}
	record("WIRE-6", "invalid params", StatusFail, 0, "expected -32602 or -32000, got %v", err)
}

// parityProbe is one call TRANSPORT-1 sends over both transports.
type parityProbe struct {
	name   string
	method string
	params any
	// result returns a fresh value to decode a success into, nil to ignore it.
	result func() any
	// ignoreMessage skips the error message, which names the transport
	// specific method path.
	ignoreMessage bool
}

// runTransportParity checks TRANSPORT-1: the same calls produce the same
// result or error on stdio and on gRPC.
func runTransportParity(ctx context.Context, stdio, grpc jsonrpc.Caller, targets conformanceTargets, record recorder) {
	probes := []parityProbe{}
	if targets.dns01Code != "" {
		probes = append(probes,
			parityProbe{
				name:   protocol.MethodDNS01Options,
				method: protocol.MethodDNS01Options,
				params: protocol.DNS01OptionsParams{Provider: targets.dns01Code, Config: map[string]string{}},
				result: func() any { return &protocol.DNS01OptionsResult{} },
			},
			parityProbe{
				name:   protocol.MethodDNS01Validate,
				method: protocol.MethodDNS01Validate,
				params: protocol.DNS01ValidateParams{Provider: targets.dns01Code, Config: map[string]string{}},
			},
		)
	}
	if targets.notify != nil {
		probes = append(probes, parityProbe{
			name:   protocol.MethodNotifyValidate,
			method: protocol.MethodNotifyValidate,
			params: protocol.NotifyValidateParams{Channel: targets.notify.Code, Config: map[string]string{}},
		})
	}
	if targets.mcpUnknownTool != "" {
		probes = append(probes, parityProbe{
			name:   protocol.MethodMCPCall + " of an unknown tool",
			method: protocol.MethodMCPCall,
			params: protocol.MCPCallParams{Tool: targets.mcpUnknownTool},
			result: func() any { return &protocol.MCPCallResult{} },
		})
	}
	probes = append(probes, parityProbe{
		name:          "unknown method",
		method:        "nginx-ui.conformance.does-not-exist",
		ignoreMessage: true,
	})

	started := time.Now()
	var names, diffs []string
	for _, probe := range probes {
		names = append(names, probe.name)
		onStdio := parityOutcome(ctx, stdio, probe)
		onGRPC := parityOutcome(ctx, grpc, probe)
		if onStdio != onGRPC {
			diffs = append(diffs, fmt.Sprintf("%s: stdio %s, grpc %s", probe.name, onStdio, onGRPC))
		}
	}
	elapsed := time.Since(started)

	if len(diffs) > 0 {
		record("TRANSPORT-1", "identical results", StatusFail, elapsed, "%s", strings.Join(diffs, "; "))
		return
	}
	record("TRANSPORT-1", "identical results", StatusPass, elapsed,
		"%s answer the same on stdio and gRPC", strings.Join(names, ", "))
}

// parityOutcome runs one probe and renders the result or the error as
// normalized JSON: a result decoded into its Go type and encoded again, an
// error as its code, data and, unless ignored, message.
func parityOutcome(ctx context.Context, caller jsonrpc.Caller, probe parityProbe) string {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var result any
	if probe.result != nil {
		result = probe.result()
	}
	err := caller.Call(callCtx, probe.method, probe.params, result)
	if err == nil {
		encoded, _ := json.Marshal(result)
		return "result " + string(encoded)
	}

	perr, ok := jsonrpc.AsProtocolError(err)
	if !ok {
		return "failure " + err.Error()
	}
	normalized := map[string]any{"code": perr.Code, "data": perr.Data}
	if !probe.ignoreMessage {
		normalized["message"] = perr.Message
	}
	encoded, _ := json.Marshal(normalized)
	return "error " + string(encoded)
}

// loadPluginForConformance resolves path to a directory and a validated
// manifest, extracting an archive into a temporary directory when needed.
func loadPluginForConformance(path string) (dir string, cleanup func(), manifest *protocol.Manifest, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, nil, err
	}

	if info.IsDir() {
		manifest, err = LoadManifest(path)
		if err != nil {
			return "", nil, nil, err
		}
		if err = ValidateManifest(manifest); err != nil {
			return "", nil, nil, err
		}
		return path, func() {}, manifest, nil
	}

	dest, err := os.MkdirTemp("", "nginx-ui-plugin-conformance-*")
	if err != nil {
		return "", nil, nil, err
	}
	pkgDir := filepath.Join(dest, "pkg")
	manifest, err = ExtractPackage(path, pkgDir)
	if err != nil {
		_ = os.RemoveAll(dest)
		return "", nil, nil, err
	}
	return pkgDir, func() { _ = os.RemoveAll(dest) }, manifest, nil
}

// runPing checks LIFE-8: the plugin answers plugin.ping.
func runPing(ctx context.Context, caller jsonrpc.Caller, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	started := time.Now()
	err := caller.Call(callCtx, protocol.MethodPing, nil, nil)
	elapsed := time.Since(started)
	if err != nil {
		record("LIFE-8", "plugin.ping", StatusFail, elapsed, "ping failed: %v", err)
		return
	}
	record("LIFE-8", "plugin.ping", StatusPass, elapsed, "answered in %s", elapsed.Round(time.Millisecond))
}

// runUnknownMethod checks WIRE-6: an unknown method answers -32601.
func runUnknownMethod(ctx context.Context, caller jsonrpc.Caller, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := caller.Call(callCtx, "nginx-ui.conformance.does-not-exist", nil, nil)
	if jsonrpc.IsMethodNotFound(err) {
		record("WIRE-6", "unknown method", StatusPass, 0, "answered -32601 as required")
		return
	}
	record("WIRE-6", "unknown method", StatusFail, 0, "expected -32601 (method not found), got %v", err)
}

// runInvalidParams checks WIRE-6: params of the wrong shape answer -32602 or
// -32000, and never hang the connection.
func runInvalidParams(ctx context.Context, caller jsonrpc.Caller, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// dns01.options expects an object; send a bare string instead.
	err := caller.Call(callCtx, protocol.MethodDNS01Options, "not-an-object", nil)
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		record("WIRE-6", "invalid params", StatusFail, 0, "the call hung instead of answering")
		return
	}
	if perr, ok := jsonrpc.AsProtocolError(err); ok && (perr.Code == protocol.CodeInvalidParams || perr.Code == protocol.CodeInternalError) {
		record("WIRE-6", "invalid params", StatusPass, 0, "malformed params answered with code %d", perr.Code)
		return
	}
	if isOptionalUnimplemented(err) {
		record("WIRE-6", "invalid params", StatusSkip, 0, "dns01.options is not implemented, cannot exercise this case")
		return
	}
	record("WIRE-6", "invalid params", StatusFail, 0, "expected -32602 or -32000, got %v", err)
}

// runNotificationThenPing checks WIRE-2: a notification for an unknown
// method is never answered, and does not break the connection.
func runNotificationThenPing(ctx context.Context, caller jsonrpc.Caller, record recorder) {
	notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := caller.Notify(notifyCtx, "nginx-ui.conformance.unknown-notification", nil)
	cancel()
	if err != nil {
		record("WIRE-2", "unanswered notification", StatusFail, 0, "sending the notification failed: %v", err)
		return
	}

	pingCtx, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	if err := caller.Call(pingCtx, protocol.MethodPing, nil, nil); err != nil {
		record("WIRE-2", "unanswered notification", StatusFail, 0, "ping right after the unknown notification failed: %v", err)
		return
	}
	record("WIRE-2", "unanswered notification", StatusPass, 0, "an unknown notification produced no reply and the connection stayed usable")
}

// runConcurrentPings checks WIRE-4: concurrent requests are all answered.
func runConcurrentPings(ctx context.Context, caller jsonrpc.Caller, record recorder) {
	const n = 20
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make([]error, n)
	started := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = caller.Call(callCtx, protocol.MethodPing, nil, nil)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(started)

	failures := 0
	for _, err := range errs {
		if err != nil {
			failures++
		}
	}
	if failures == 0 {
		record("WIRE-4", "concurrent pings", StatusPass, elapsed, "%d concurrent plugin.ping calls all answered", n)
		return
	}
	record("WIRE-4", "concurrent pings", StatusFail, elapsed, "%d of %d concurrent pings failed", failures, n)
}

// runStdoutHygiene approximates WIRE-1 (stdout carries protocol frames
// only): the supervisor already closes the connection on a malformed line,
// so instead this checks the plugin actually used stderr for at least one
// line, which is the positive half of the same requirement.
func runStdoutHygiene(sup *Supervisor, record recorder) {
	logs := sup.Logs()
	if len(logs) == 0 {
		record("WIRE-1", "stdout hygiene", StatusSkip, 0, "not observable: the plugin wrote nothing to stderr during this run")
		return
	}
	record("WIRE-1", "stdout hygiene", StatusPass, 0, "the plugin wrote %d line(s) to stderr, consistent with keeping stdout for protocol frames only", len(logs))
}

// effectiveCapabilities intersects the manifest's own declared capabilities
// with the ones the caller asked to test, when it asked to limit them.
func effectiveCapabilities(manifest *protocol.Manifest, opts ConformanceOptions) []string {
	if len(opts.Capabilities) == 0 {
		return manifest.Capabilities
	}
	out := make([]string, 0, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		if slices.Contains(opts.Capabilities, capability) {
			out = append(out, capability)
		}
	}
	return out
}

// runDNS01Cases exercises the optional dns01 methods against the manifest's
// first declared provider, per spec/09-conformance.md CONF-2.
func runDNS01Cases(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	runDNS01Options(ctx, caller, code, record)
	runDNS01Validate(ctx, caller, code, record)
	runDNS01Present(ctx, caller, code, record)
	runDNS01Check(ctx, caller, code, record)
}

// isOptionalUnimplemented reports whether err is how a plugin says "I do not
// implement this optional method", per DNS01-9/10/11: -32002 is the spec
// answer, but a plugin registering no handler at all answers -32601, which
// the spec says a host must treat the same way.
func isOptionalUnimplemented(err error) bool {
	return jsonrpc.IsMethodNotFound(err) || jsonrpc.IsUnsupported(err)
}

func protocolErrorCode(err error) string {
	if perr, ok := jsonrpc.AsProtocolError(err); ok {
		return fmt.Sprintf("code %d", perr.Code)
	}
	return "no protocol error"
}

// runDNS01Options checks DNS01-10: an empty config either answers, or
// reports the method is not implemented.
func runDNS01Options(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var result protocol.DNS01OptionsResult
	err := caller.Call(callCtx, protocol.MethodDNS01Options, protocol.DNS01OptionsParams{Provider: code, Config: map[string]string{}}, &result)
	switch {
	case err == nil:
		record("DNS01-10", "dns01.options", StatusPass, 0,
			"implemented: propagation_timeout_seconds=%d polling_interval_seconds=%d", result.PropagationTimeoutSeconds, result.PollingIntervalSeconds)
	case isOptionalUnimplemented(err):
		record("DNS01-10", "dns01.options", StatusSkip, 0, "not implemented (%s); the host falls back to its own defaults", protocolErrorCode(err))
	case isInvalidConfig(err):
		record("DNS01-10", "dns01.options", StatusPass, 0, "an empty config was rejected with -32003; the provider needs credentials to answer")
	default:
		record("DNS01-10", "dns01.options", StatusFail, 0, "unexpected error: %v", err)
	}
}

// runDNS01Validate checks DNS01-9: an empty config is rejected with
// CodeInvalidConfig, or the method is reported as not implemented.
func runDNS01Validate(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := caller.Call(callCtx, protocol.MethodDNS01Validate, protocol.DNS01ValidateParams{Provider: code, Config: map[string]string{}}, nil)
	perr, ok := jsonrpc.AsProtocolError(err)
	switch {
	case ok && perr.Code == protocol.CodeInvalidConfig:
		record("DNS01-9", "dns01.validate", StatusPass, 0, "an empty config was rejected with -32003 as required")
	case isOptionalUnimplemented(err):
		record("DNS01-9", "dns01.validate", StatusSkip, 0, "not implemented (%s)", protocolErrorCode(err))
	default:
		record("DNS01-9", "dns01.validate", StatusFail, 0, "expected -32003 for an empty config, got %v", err)
	}
}

// runDNS01Present checks DNS01-4: a dry_run present with an empty config
// must not hang and must return within 30 seconds. Any result is acceptable.
func runDNS01Present(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	err := caller.Call(callCtx, protocol.MethodDNS01Present, protocol.DNS01ChallengeParams{
		Provider:      code,
		Config:        map[string]string{},
		Domain:        "conformance.invalid",
		FQDN:          "_acme-challenge.conformance.invalid.",
		EffectiveFQDN: "_acme-challenge.conformance.invalid.",
		Value:         "conformance-value",
		Token:         "conformance-token",
		KeyAuth:       "conformance-token.conformance-thumbprint",
		DryRun:        true,
	}, nil)
	elapsed := time.Since(started)
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		record("DNS01-4", "dns01.present dry_run", StatusFail, elapsed, "timed out waiting for a reply")
		return
	}
	record("DNS01-4", "dns01.present dry_run", StatusPass, elapsed,
		"answered in %s (err=%v, any result is acceptable for a dry run)", elapsed.Round(time.Millisecond), err)
}

// runDNS01Check checks DNS01-11: an empty config must return within 30
// seconds, or the method is reported as not implemented.
func runDNS01Check(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	var result protocol.DNS01CheckResult
	err := caller.Call(callCtx, protocol.MethodDNS01Check, protocol.DNS01CheckParams{
		Provider: code,
		Config:   map[string]string{},
		Domain:   "conformance.invalid",
		FQDN:     "_acme-challenge.conformance.invalid.",
		Value:    "conformance-value",
		KeyAuth:  "conformance-token.conformance-thumbprint",
	}, &result)
	elapsed := time.Since(started)
	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		record("DNS01-11", "dns01.check", StatusFail, elapsed, "timed out waiting for a reply")
		return
	}
	if isOptionalUnimplemented(err) {
		record("DNS01-11", "dns01.check", StatusSkip, elapsed, "not implemented (%s)", protocolErrorCode(err))
		return
	}
	record("DNS01-11", "dns01.check", StatusPass, elapsed, "answered in %s (err=%v)", elapsed.Round(time.Millisecond), err)
}

// requiredConfigurationField returns the key of the first required field of
// a form, empty when nothing is required.
func requiredConfigurationField(schema *protocol.ConfigurationSchema) string {
	if schema == nil {
		return ""
	}
	for _, field := range schema.Fields {
		if field.Required {
			return field.Key
		}
	}
	return ""
}

// invalidConfigFieldOf returns data.field of a CodeInvalidConfig error.
func invalidConfigFieldOf(err error) (field string, ok bool) {
	perr, isProtocol := jsonrpc.AsProtocolError(err)
	if !isProtocol || perr.Code != protocol.CodeInvalidConfig {
		return "", false
	}
	switch data := perr.Data.(type) {
	case map[string]any:
		field, _ = data["field"].(string)
	case protocol.InvalidConfigData:
		field = data.Field
	case *protocol.InvalidConfigData:
		if data != nil {
			field = data.Field
		}
	}
	return field, true
}

// runNotifyValidate checks NOTIFY-8 against the manifest's first channel: an
// empty config is rejected with -32003 and data.field when the channel has a
// required field, and accepted otherwise. notify.send is never called, it
// would reach the vendor.
func runNotifyValidate(ctx context.Context, caller jsonrpc.Caller, channel protocol.NotifyChannel, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := caller.Call(callCtx, protocol.MethodNotifyValidate, protocol.NotifyValidateParams{
		Channel: channel.Code,
		Config:  map[string]string{},
	}, nil)

	required := requiredConfigurationField(channel.Configuration)
	field, invalid := invalidConfigFieldOf(err)
	switch {
	case isOptionalUnimplemented(err):
		record("NOTIFY-8", "notify.validate", StatusSkip, 0, "not implemented (%s)", protocolErrorCode(err))
	case invalid && field == "":
		record("NOTIFY-8", "notify.validate", StatusFail, 0, "an empty config was rejected with -32003 but without data.field")
	case invalid:
		record("NOTIFY-8", "notify.validate", StatusPass, 0, "an empty config for channel %s was rejected with -32003 naming %s", channel.Code, field)
	case err == nil && required != "":
		record("NOTIFY-8", "notify.validate", StatusFail, 0, "an empty config was accepted although field %s is required", required)
	case err == nil:
		record("NOTIFY-8", "notify.validate", StatusPass, 0, "channel %s declares no required field and accepted an empty config", channel.Code)
	default:
		record("NOTIFY-8", "notify.validate", StatusFail, 0, "expected -32003 or {}, got %v", err)
	}
}

// conformanceProbeTarget is unroutable, so a probe answers without touching
// anything real.
const conformanceProbeTarget = "http://conformance.invalid"

// runProbeCheck checks PROBE-4 and PROBE-5 against the manifest's first kind:
// the unroutable target answers in time with a known status, or the empty
// config is rejected with -32003 and data.field.
func runProbeCheck(ctx context.Context, caller jsonrpc.Caller, code string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	started := time.Now()
	var result protocol.ProbeCheckResult
	err := caller.Call(callCtx, protocol.MethodProbeCheck, protocol.ProbeCheckParams{
		Kind:           code,
		Target:         conformanceProbeTarget,
		Config:         map[string]string{},
		TimeoutSeconds: 5,
	}, &result)
	elapsed := time.Since(started)

	if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		record("PROBE-4", "probe.check", StatusFail, elapsed, "no answer within 15 seconds for timeout_seconds 5")
		return
	}
	if field, invalid := invalidConfigFieldOf(err); invalid {
		if field == "" {
			record("PROBE-5", "probe.check", StatusFail, elapsed, "an empty config was rejected with -32003 but without data.field")
			return
		}
		record("PROBE-5", "probe.check", StatusPass, elapsed, "an empty config for kind %s was rejected with -32003 naming %s", code, field)
		return
	}
	if err != nil {
		record("PROBE-5", "probe.check", StatusFail, elapsed, "an unreachable target must be reported as down, got %v", err)
		return
	}
	switch result.Status {
	case protocol.ProbeStatusUp, protocol.ProbeStatusDown, protocol.ProbeStatusDegraded:
	default:
		record("PROBE-5", "probe.check", StatusFail, elapsed, "unknown status %q", result.Status)
		return
	}
	if result.LatencyMS < 0 {
		record("PROBE-5", "probe.check", StatusFail, elapsed, "negative latency_ms %d", result.LatencyMS)
		return
	}
	record("PROBE-5", "probe.check", StatusPass, elapsed, "%s answered %s in %s", conformanceProbeTarget, result.Status, elapsed.Round(time.Millisecond))
}

// runMCPUnknownTool checks MCP-6: a tool the manifest does not declare answers
// -32602. No declared tool is called, it may change state.
func runMCPUnknownTool(ctx context.Context, caller jsonrpc.Caller, tool string, record recorder) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := caller.Call(callCtx, protocol.MethodMCPCall, protocol.MCPCallParams{Tool: tool}, &protocol.MCPCallResult{})
	if perr, ok := jsonrpc.AsProtocolError(err); ok && perr.Code == protocol.CodeInvalidParams {
		record("MCP-6", "mcp.call unknown tool", StatusPass, 0, "an unknown tool answered -32602 as required")
		return
	}
	if err == nil {
		record("MCP-6", "mcp.call unknown tool", StatusFail, 0, "tool %s is not declared but answered with a result", tool)
		return
	}
	record("MCP-6", "mcp.call unknown tool", StatusFail, 0, "expected -32602 for an unknown tool, got %v", err)
}

// checkWebapp checks WEB-1 and WEB-5 against the bundle file on disk. It
// needs no running plugin process.
func checkWebapp(dir string, manifest *protocol.Manifest, record recorder) {
	if manifest.Webapp == nil || manifest.Webapp.BundlePath == "" {
		return
	}

	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(manifest.Webapp.BundlePath)))
	if err != nil {
		record("WEB-1", "webapp bundle", StatusFail, 0, "cannot read the bundle: %v", err)
		return
	}

	if len(data) > maxWebappBundleSize {
		record("WEB-1", "webapp bundle size", StatusFail, 0, "bundle is %d bytes, more than the %d byte budget", len(data), maxWebappBundleSize)
	} else {
		record("WEB-1", "webapp bundle size", StatusPass, 0, "bundle is %d bytes", len(data))
	}

	text := string(data)
	if strings.Contains(text, "registerPlugin(") {
		record("WEB-5", "webapp registerPlugin", StatusPass, 0, "the bundle calls registerPlugin(")
	} else {
		record("WEB-5", "webapp registerPlugin", StatusFail, 0, "the bundle never calls registerPlugin(")
	}

	if strings.Contains(text, "createApp(") {
		record("WEB-1", "webapp createApp", StatusWarn, 0, "the bundle calls createApp(, which WEB-1 forbids")
	} else {
		record("WEB-1", "webapp createApp", StatusPass, 0, "the bundle does not call createApp(")
	}

	if strings.Contains(text, "eval(") {
		record("WEB-1", "webapp eval", StatusFail, 0, "the bundle calls eval(")
	} else {
		record("WEB-1", "webapp eval", StatusPass, 0, "the bundle does not call eval(")
	}
}

// conformanceBackend is a minimal HostBackend: it records host.log calls and
// answers every other host.* method with an empty, harmless value.
type conformanceBackend struct {
	mu   sync.Mutex
	logs []protocol.HostLogParams
}

func (b *conformanceBackend) Log(_ string, p protocol.HostLogParams) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logs = append(b.logs, p)
}
func (b *conformanceBackend) KVGet(string, string) (json.RawMessage, bool, error) {
	return nil, false, nil
}
func (b *conformanceBackend) KVSet(string, string, json.RawMessage) error { return nil }
func (b *conformanceBackend) KVDelete(string, string) error               { return nil }
func (b *conformanceBackend) KVList(string, string) ([]string, error)     { return nil, nil }
func (b *conformanceBackend) SettingsGet(string) (map[string]any, error) {
	return map[string]any{}, nil
}
func (b *conformanceBackend) Locale() string { return "en" }
func (b *conformanceBackend) CredentialsGet(string, string, string) (*protocol.HostCredentialsGetResult, error) {
	return nil, nil
}
func (b *conformanceBackend) CronRegister(string, protocol.HostCronRegisterParams) error { return nil }
func (b *conformanceBackend) CronUnregister(string, string) error                        { return nil }
func (b *conformanceBackend) Notify(string, protocol.HostNotifyParams) error             { return nil }
func (b *conformanceBackend) MetricsSnapshot() (any, error)                              { return nil, nil }

// isInvalidConfig reports whether err is the CodeInvalidConfig protocol error.
func isInvalidConfig(err error) bool {
	perr, ok := jsonrpc.AsProtocolError(err)
	return ok && perr.Code == protocol.CodeInvalidConfig
}
