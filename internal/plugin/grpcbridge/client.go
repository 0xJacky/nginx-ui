package grpcbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultSocketName is the socket file inside the plugin data directory used
// when the plugin does not report rpc_socket.
const DefaultSocketName = "rpc.sock"

// MaxMessageBytes bounds one gRPC message in either direction. gRPC exists
// for payloads the 4 MiB stdio frame cannot carry, so it is larger.
const MaxMessageBytes = 64 << 20

// Endpoint is where a plugin serves gRPC.
type Endpoint struct {
	// Socket is the Unix socket path. Empty when Port is set.
	Socket string
	// Port is the loopback TCP port Windows plugins listen on.
	Port int
	// Token is sent as "authorization: Bearer <token>" on the TCP endpoint.
	Token string
}

// EndpointFor reads the endpoint out of the handshake. ok is false when the
// plugin does not advertise the grpc transport.
func EndpointFor(result protocol.InitializeResult, dataDir string) (Endpoint, bool) {
	if !slices.Contains(result.Transports, protocol.TransportGRPC) {
		return Endpoint{}, false
	}
	if result.RPCPort > 0 {
		return Endpoint{Port: result.RPCPort, Token: result.RPCToken}, true
	}
	socket := result.RPCSocket
	if socket == "" {
		socket = filepath.Join(dataDir, DefaultSocketName)
	}
	if abs, err := filepath.Abs(socket); err == nil {
		socket = abs
	}
	return Endpoint{Socket: socket}, true
}

// Target is the gRPC dial target of the endpoint.
func (e Endpoint) Target() string {
	if e.Port > 0 {
		return net.JoinHostPort("127.0.0.1", strconv.Itoa(e.Port))
	}
	return "unix://" + e.Socket
}

func (e Endpoint) String() string { return e.Target() }

// bearerToken attaches the loopback token to every call.
type bearerToken string

func (t bearerToken) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(t)}, nil
}

// RequireTransportSecurity is false: the token only guards a loopback port.
func (bearerToken) RequireTransportSecurity() bool { return false }

// Client is a gRPC channel to one plugin process.
type Client struct {
	conn      *grpc.ClientConn
	closeOnce sync.Once
	closeErr  error
}

// Dial connects to the endpoint and waits until the channel is ready or ctx
// ends. opts are added to the bridge defaults.
func Dial(ctx context.Context, ep Endpoint, opts ...grpc.DialOption) (*Client, error) {
	if ep.Token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearerToken(ep.Token)))
	}
	return DialTarget(ctx, ep.Target(), opts...)
}

// DialTarget connects to an arbitrary target with the bridge defaults plus
// opts, and waits until the channel is ready or ctx ends. A channel that
// fails to connect is reported at once instead of retried: a plugin opens its
// listener before it answers the handshake.
func DialTarget(ctx context.Context, target string, opts ...grpc.DialOption) (*Client, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(RawCodec{}),
			grpc.MaxCallRecvMsgSize(MaxMessageBytes),
			grpc.MaxCallSendMsgSize(MaxMessageBytes),
		),
	}
	conn, err := grpc.NewClient(target, append(base, opts...)...)
	if err != nil {
		return nil, err
	}

	conn.Connect()
	for {
		state := conn.GetState()
		switch state {
		case connectivity.Ready:
			return &Client{conn: conn}, nil
		case connectivity.TransientFailure, connectivity.Shutdown:
			_ = conn.Close()
			return nil, fmt.Errorf("%w: connecting to %s failed (%s)", ErrUnavailable, target, state)
		}
		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, fmt.Errorf("%w: connecting to %s: %v", ErrUnavailable, target, ctx.Err())
		}
	}
}

// Call sends a JSON-RPC shaped call over gRPC. params are converted to the
// request message of method and the response message is decoded into result,
// which may be nil. Errors match what the stdio transport returns.
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	m, ok := Lookup(method)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotInContract, method)
	}
	if m.Streaming {
		return fmt.Errorf("%w: %s", ErrStreamingMethod, method)
	}

	in, err := EncodeJSON(m.Input, params)
	if err != nil {
		return &protocol.Error{Code: protocol.CodeInvalidParams, Message: fmt.Sprintf("invalid params for %s: %v", method, err)}
	}

	out, err := c.Invoke(ctx, m.FullMethod, in)
	if err != nil {
		return FromStatus(ctx, err, true)
	}
	if result == nil {
		return nil
	}

	raw, err := DecodeJSON(m.Output, out)
	if err != nil {
		return fmt.Errorf("grpcbridge: decode result of %s: %w", method, err)
	}
	if err = json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("grpcbridge: decode result of %s: %w", method, err)
	}
	return nil
}

// Invoke sends raw request bytes to a gRPC path and returns the raw response.
// The error is the gRPC status, see FromStatus.
func (c *Client) Invoke(ctx context.Context, fullMethod string, in []byte) ([]byte, error) {
	var out []byte
	if err := c.conn.Invoke(ctx, fullMethod, &in, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// State reports the connectivity state of the channel.
func (c *Client) State() connectivity.State { return c.conn.GetState() }

// Connect asks an idle channel to reconnect.
func (c *Client) Connect() { c.conn.Connect() }

// WaitForStateChange blocks until the state differs from s or ctx ends.
func (c *Client) WaitForStateChange(ctx context.Context, s connectivity.State) bool {
	return c.conn.WaitForStateChange(ctx, s)
}

// Close tears the channel down. Calls still running fail. It is safe to call
// more than once.
func (c *Client) Close() error {
	c.closeOnce.Do(func() { c.closeErr = c.conn.Close() })
	return c.closeErr
}
