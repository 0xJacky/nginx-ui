package plugin

// This file routes capability calls over the optional gRPC transport
// (docs/plugin/protocol.md). stdio stays the
// baseline: the handshake, lifecycle methods, host API calls, logs and
// notifications never leave it, and every capability call falls back to it
// when the gRPC channel cannot be reached.

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"go.uber.org/zap"
	"google.golang.org/grpc/connectivity"
)

// defaultGRPCDialTimeout bounds connecting to the gRPC endpoint and probing
// it with plugin.ping after the handshake.
const defaultGRPCDialTimeout = 5 * time.Second

// grpcLossGrace is how long a lost channel waits for the process to exit
// before it is reported, so a crash or a stop is not logged twice.
const grpcLossGrace = 500 * time.Millisecond

// grpcRoute is the gRPC side of one plugin process.
type grpcRoute struct {
	pluginID string
	log      *zap.SugaredLogger
	endpoint grpcbridge.Endpoint

	client atomic.Pointer[grpcbridge.Client]
	// stopping marks a deliberate stop, the channel going away is expected.
	stopping atomic.Bool
}

// clientFor returns the gRPC client when method is a capability rpc and the
// channel is up, nil when the call belongs on stdio.
func (r *grpcRoute) clientFor(method string) *grpcbridge.Client {
	if r == nil {
		return nil
	}
	client := r.client.Load()
	if client == nil || !grpcbridge.IsCapability(method) {
		return nil
	}
	return client
}

// active reports whether capability calls currently travel over gRPC.
func (r *grpcRoute) active() bool {
	return r != nil && r.client.Load() != nil
}

// drop switches the process back to stdio for good. Only the call that
// actually removes the client logs, so the switch is reported once.
func (r *grpcRoute) drop(cause error, quiet bool) {
	if r == nil {
		return
	}
	client := r.client.Swap(nil)
	if client == nil {
		return
	}
	_ = client.Close()
	if quiet || r.stopping.Load() {
		r.log.Debugf("[plugin:%s] gRPC transport closed: %v", r.pluginID, cause)
		return
	}
	r.log.Warnf("[plugin:%s] gRPC transport at %s lost, capability calls fall back to stdio: %v",
		r.pluginID, r.endpoint, cause)
}

// close releases the channel once the process is gone.
func (r *grpcRoute) close() {
	r.drop(errors.New("plugin process exited"), true)
}

// markStopping records a deliberate stop so losing the channel stays quiet.
func (r *grpcRoute) markStopping() {
	if r != nil {
		r.stopping.Store(true)
	}
}

// connectGRPC dials the endpoint the plugin advertised and verifies it with
// plugin.ping. Any failure leaves the process on stdio.
func (s *Supervisor) connectGRPC(ctx context.Context, p *process) {
	route := &grpcRoute{pluginID: s.cfg.PluginID, log: s.log}
	p.grpc = route

	endpoint, ok := grpcbridge.EndpointFor(p.initResult, s.cfg.DataDir)
	if !ok {
		return
	}
	route.endpoint = endpoint

	dialCtx, cancel := context.WithTimeout(ctx, s.grpcDialTimeout)
	defer cancel()

	client, err := grpcbridge.Dial(dialCtx, endpoint)
	if err == nil {
		if err = client.Call(dialCtx, protocol.MethodPing, nil, nil); err != nil {
			_ = client.Close()
		}
	}
	if err != nil {
		s.log.Warnf("[plugin:%s] gRPC transport at %s unavailable, capability calls use stdio: %v",
			s.cfg.PluginID, endpoint, err)
		return
	}

	route.client.Store(client)
	s.log.Debugf("[plugin:%s] capability calls use gRPC at %s", s.cfg.PluginID, endpoint)
	go s.watchGRPC(p, route, client)
}

// watchGRPC notices a channel that went away without a call running into it.
// An idle channel is asked to reconnect, which fails fast when the plugin
// closed its listener.
func (s *Supervisor) watchGRPC(p *process, route *grpcRoute, client *grpcbridge.Client) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-p.done:
		case <-p.conn.Done():
		case <-ctx.Done():
		}
		cancel()
	}()

	state := client.State()
	for {
		switch state {
		case connectivity.Shutdown:
			return
		case connectivity.Idle:
			client.Connect()
		case connectivity.TransientFailure:
			// A dying process takes its listener with it; that exit is
			// reported by the supervisor already.
			select {
			case <-p.done:
				route.drop(errors.New("plugin process exited"), true)
			case <-p.conn.Done():
				route.drop(errors.New("plugin process exited"), true)
			case <-time.After(grpcLossGrace):
				route.drop(errors.New("the plugin closed its gRPC listener"), false)
			}
			return
		}
		if !client.WaitForStateChange(ctx, state) {
			return
		}
		state = client.State()
	}
}

// Transport reports how capability calls reach the running plugin, "grpc" or
// "stdio", and "" when no process is running.
func (s *Supervisor) Transport() string {
	s.mu.Lock()
	p := s.proc
	running := p != nil && s.state == StateRunning
	s.mu.Unlock()
	if !running {
		return ""
	}
	if p.grpc.active() {
		return protocol.TransportGRPC
	}
	return protocol.TransportStdio
}

// stdioClient returns a caller for the running plugin that never uses gRPC.
func (s *Supervisor) stdioClient() (jsonrpc.Caller, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil || s.state != StateRunning {
		return nil, ErrPluginNotRunning
	}
	return &timeoutCaller{conn: s.proc.conn, timeout: s.callTimeout}, nil
}

// timeoutCaller applies the default per call timeout to callers that did not
// set a deadline of their own, and sends capability calls over gRPC when the
// plugin serves it.
type timeoutCaller struct {
	conn    *jsonrpc.Conn
	timeout time.Duration
	// route is nil for a stdio only caller.
	route *grpcRoute
}

func (c *timeoutCaller) Call(ctx context.Context, method string, params any, result any) error {
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	err := c.call(ctx, method, params, result)
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrCallTimeout
	}
	return err
}

func (c *timeoutCaller) call(ctx context.Context, method string, params any, result any) error {
	if client := c.route.clientFor(method); client != nil {
		err := client.Call(ctx, method, params, result)
		if !errors.Is(err, grpcbridge.ErrUnavailable) {
			return err
		}
		// The channel broke under this call; the process still answers on
		// stdio, so the call is retried there.
		c.route.drop(err, false)
	}
	return c.conn.Call(ctx, method, params, result)
}

func (c *timeoutCaller) Notify(ctx context.Context, method string, params any) error {
	return c.conn.Notify(ctx, method, params)
}
