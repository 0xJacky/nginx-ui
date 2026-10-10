package grpcbridge_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge/bridgetest"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// startServer serves srv on an in-memory listener and returns a connected
// client.
func startServer(t *testing.T, srv *grpc.Server, ep grpcbridge.Endpoint) *grpcbridge.Client {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	if ep.Port == 0 && ep.Socket == "" {
		ep.Port = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := grpcbridge.Dial(ctx, ep, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// fakePlugin answers like a dns01 plugin and records the params it saw.
type fakePlugin struct {
	seen     chan json.RawMessage
	deadline chan time.Time
}

func newFakePlugin() *fakePlugin {
	return &fakePlugin{seen: make(chan json.RawMessage, 8), deadline: make(chan time.Time, 1)}
}

func (f *fakePlugin) handle(ctx context.Context, method string, params json.RawMessage) (any, error) {
	f.seen <- params
	switch method {
	case protocol.MethodDNS01Options:
		return protocol.DNS01OptionsResult{PropagationTimeoutSeconds: 120, PollingIntervalSeconds: 2}, nil
	case protocol.MethodDNS01Validate:
		return nil, &protocol.Error{Code: protocol.CodeInvalidConfig, Message: "TOKEN is required", Data: protocol.InvalidConfigData{Field: "TOKEN"}}
	case protocol.MethodDNS01Check:
		if dl, ok := ctx.Deadline(); ok {
			f.deadline <- dl
		}
		<-ctx.Done()
		return nil, ctx.Err()
	case protocol.MethodDNS01Present:
		return nil, errors.New("vendor down")
	default:
		return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method: " + method}
	}
}

func TestCallConvertsParamsAndResult(t *testing.T) {
	plugin := newFakePlugin()
	client := startServer(t, bridgetest.NewServer(plugin.handle), grpcbridge.Endpoint{})

	var result protocol.DNS01OptionsResult
	err := client.Call(context.Background(), protocol.MethodDNS01Options, protocol.DNS01OptionsParams{
		Provider: "cloudflare",
		Config:   map[string]string{"CF_DNS_API_TOKEN": "secret"},
		Options:  map[string]any{"credential_id": "7", "disable_cname": true},
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, protocol.DNS01OptionsResult{PropagationTimeoutSeconds: 120, PollingIntervalSeconds: 2}, result)

	// The plugin side sees the JSON-RPC params with proto field names.
	var params protocol.DNS01OptionsParams
	require.NoError(t, json.Unmarshal(<-plugin.seen, &params))
	assert.Equal(t, "cloudflare", params.Provider)
	assert.Equal(t, "secret", params.Config["CF_DNS_API_TOKEN"])
	assert.Equal(t, map[string]any{"credential_id": "7", "disable_cname": true}, params.Options)

	// A nil result discards the reply, as on stdio.
	require.NoError(t, client.Call(context.Background(), protocol.MethodDNS01Options, nil, nil))
}

func TestCallReturnsThePluginErrorDetail(t *testing.T) {
	client := startServer(t, bridgetest.NewServer(newFakePlugin().handle), grpcbridge.Endpoint{})

	err := client.Call(context.Background(), protocol.MethodDNS01Validate, protocol.DNS01ValidateParams{Provider: "x"}, nil)
	var perr *protocol.Error
	require.ErrorAs(t, err, &perr)
	// The same value encoding/json produces from the stdio error object.
	assert.Equal(t, &protocol.Error{
		Code:    protocol.CodeInvalidConfig,
		Message: "TOKEN is required",
		Data:    map[string]any{"field": "TOKEN"},
	}, perr)

	err = client.Call(context.Background(), protocol.MethodDNS01Present, protocol.DNS01ChallengeParams{}, nil)
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, protocol.CodeInternalError, perr.Code)
	assert.Equal(t, "vendor down", perr.Message)
}

func TestCallRespectsTheCallerDeadline(t *testing.T) {
	plugin := newFakePlugin()
	client := startServer(t, bridgetest.NewServer(plugin.handle), grpcbridge.Endpoint{})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := client.Call(ctx, protocol.MethodDNS01Check, protocol.DNS01CheckParams{}, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), 3*time.Second)

	// The deadline travelled to the plugin.
	select {
	case dl := <-plugin.deadline:
		want, _ := ctx.Deadline()
		assert.WithinDuration(t, want, dl, time.Second)
	case <-time.After(time.Second):
		t.Fatal("the plugin saw no deadline")
	}

	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	assert.ErrorIs(t, client.Call(cancelled, protocol.MethodDNS01Check, nil, nil), context.Canceled)
}

func TestCallRejectsParamsThatDoNotFitTheRequest(t *testing.T) {
	client := startServer(t, bridgetest.NewServer(newFakePlugin().handle), grpcbridge.Endpoint{})

	err := client.Call(context.Background(), protocol.MethodDNS01Options, "not-an-object", nil)
	var perr *protocol.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, protocol.CodeInvalidParams, perr.Code)

	err = client.Call(context.Background(), "nginx-ui.test.nope", nil, nil)
	assert.ErrorIs(t, err, grpcbridge.ErrNotInContract)
}

// statusServer answers every path with the status of the matching method.
func statusServer(byPath map[string]*status.Status) *grpc.Server {
	return grpc.NewServer(
		grpc.ForceServerCodec(grpcbridge.RawCodec{}),
		grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			path, _ := grpc.MethodFromServerStream(stream)
			var in []byte
			if err := stream.RecvMsg(&in); err != nil {
				return err
			}
			if st, ok := byPath[path]; ok {
				return st.Err()
			}
			return status.Error(codes.Unimplemented, "unknown service")
		}),
	)
}

func TestFromStatusWithoutDetailUsesTheTable(t *testing.T) {
	cases := map[codes.Code]int{
		codes.InvalidArgument:  protocol.CodeInvalidParams,
		codes.Unimplemented:    protocol.CodeUnsupported,
		codes.PermissionDenied: protocol.CodePermissionDenied,
		codes.Unauthenticated:  protocol.CodePermissionDenied,
		codes.Internal:         protocol.CodeInternalError,
		codes.NotFound:         protocol.CodeInternalError,
	}
	for code, want := range cases {
		t.Run(code.String(), func(t *testing.T) {
			m, ok := grpcbridge.Lookup(protocol.MethodDNS01Check)
			require.True(t, ok)
			client := startServer(t, statusServer(map[string]*status.Status{
				m.FullMethod: status.New(code, "bare "+code.String()),
			}), grpcbridge.Endpoint{})

			err := client.Call(context.Background(), protocol.MethodDNS01Check, nil, nil)
			var perr *protocol.Error
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, want, perr.Code)
			assert.Equal(t, "bare "+code.String(), perr.Message)
		})
	}

	t.Run("outside the contract", func(t *testing.T) {
		client := startServer(t, statusServer(nil), grpcbridge.Endpoint{})
		ctx := context.Background()
		_, err := client.Invoke(ctx, "/nginxui.plugin.v1.Conformance/Nope", nil)
		var perr *protocol.Error
		require.ErrorAs(t, grpcbridge.FromStatus(ctx, err, false), &perr)
		assert.Equal(t, protocol.CodeMethodNotFound, perr.Code)
	})

	t.Run("unavailable", func(t *testing.T) {
		m, _ := grpcbridge.Lookup(protocol.MethodDNS01Check)
		client := startServer(t, statusServer(map[string]*status.Status{
			m.FullMethod: status.New(codes.Unavailable, "gone"),
		}), grpcbridge.Endpoint{})
		err := client.Call(context.Background(), protocol.MethodDNS01Check, nil, nil)
		assert.ErrorIs(t, err, grpcbridge.ErrUnavailable)
	})
}

func TestDialSendsTheLoopbackToken(t *testing.T) {
	const token = "0123456789abcdef"
	seen := make(chan string, 1)
	srv := bridgetest.NewServer(func(ctx context.Context, method string, _ json.RawMessage) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		seen <- first(md.Get("authorization"))
		return protocol.EmptyResult{}, nil
	})
	client := startServer(t, srv, grpcbridge.Endpoint{Port: 43210, Token: token})

	require.NoError(t, client.Call(context.Background(), protocol.MethodPing, nil, nil))
	assert.Equal(t, "Bearer "+token, <-seen)
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func TestDialFailsFastWhenNothingListens(t *testing.T) {
	dir, err := os.MkdirTemp("", "gbt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	started := time.Now()
	_, err = grpcbridge.Dial(ctx, grpcbridge.Endpoint{Socket: filepath.Join(dir, "missing.sock")})
	assert.ErrorIs(t, err, grpcbridge.ErrUnavailable)
	assert.Less(t, time.Since(started), 3*time.Second)
}

func TestDialOverAUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "gbt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socket := filepath.Join(dir, grpcbridge.DefaultSocketName)
	lis, err := net.Listen("unix", socket)
	require.NoError(t, err)
	srv := bridgetest.NewServer(newFakePlugin().handle)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	ep, ok := grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"stdio", "grpc"}}, dir)
	require.True(t, ok)
	assert.Equal(t, "unix://"+socket, ep.Target())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := grpcbridge.Dial(ctx, ep)
	require.NoError(t, err)
	defer client.Close()

	var result protocol.DNS01OptionsResult
	require.NoError(t, client.Call(ctx, protocol.MethodDNS01Options, protocol.DNS01OptionsParams{Provider: "x"}, &result))
	assert.Equal(t, 120, result.PropagationTimeoutSeconds)
}

func TestEndpointFor(t *testing.T) {
	_, ok := grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"stdio"}}, "/data")
	assert.False(t, ok, "stdio only")

	ep, ok := grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"grpc"}, RPCSocket: "/tmp/x/rpc.sock"}, "/data")
	require.True(t, ok)
	assert.Equal(t, "unix:///tmp/x/rpc.sock", ep.Target())

	ep, ok = grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"grpc"}, RPCPort: 5000, RPCToken: "t"}, "/data")
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1:5000", ep.Target())
	assert.Equal(t, "t", ep.Token)

	// A pipe wins over a port, and its name stays out of the target.
	ep, ok = grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"grpc"},
		RPCPipe: `\\.\pipe\nginx-ui-rpc-1`, RPCPort: 5000, RPCToken: "t"}, "/data")
	require.True(t, ok)
	assert.Equal(t, `\\.\pipe\nginx-ui-rpc-1`, ep.Pipe)
	assert.Equal(t, "passthrough:///plugin", ep.Target())
	assert.Equal(t, `\\.\pipe\nginx-ui-rpc-1`, ep.String())
	assert.Equal(t, "t", ep.Token)

	ep, ok = grpcbridge.EndpointFor(protocol.InitializeResult{Transports: []string{"grpc"}}, "/data")
	require.True(t, ok)
	assert.Equal(t, "unix:///data/rpc.sock", ep.Target())
}

func TestOnlyCapabilityRPCsAreRoutable(t *testing.T) {
	for _, method := range []string{protocol.MethodDNS01Present, protocol.MethodDNS01Cleanup, protocol.MethodDNS01Options,
		protocol.MethodDNS01Check, protocol.MethodDNS01Validate, protocol.MethodHTTPHandle} {
		assert.True(t, grpcbridge.IsCapability(method), method)
	}
	for _, method := range []string{protocol.MethodInitialize, protocol.MethodPing, protocol.MethodShutdown,
		protocol.MethodExit, protocol.MethodEventsOn, protocol.MethodHostKVGet, protocol.MethodHostLog, "catalog.refresh"} {
		assert.False(t, grpcbridge.IsCapability(method), method)
	}

	m, ok := grpcbridge.Lookup(protocol.MethodDNS01Present)
	require.True(t, ok)
	assert.Equal(t, "/nginxui.plugin.v1.DNS01/Present", m.FullMethod)
	same, ok := grpcbridge.LookupFullMethod(m.FullMethod)
	require.True(t, ok)
	assert.Same(t, m, same)
}

func TestRawCodecRoundTrip(t *testing.T) {
	codec := grpcbridge.RawCodec{}
	assert.Equal(t, "proto", codec.Name())

	payload := []byte{1, 2, 3}
	out, err := codec.Marshal(&payload)
	require.NoError(t, err)
	assert.Equal(t, payload, out)

	var got []byte
	require.NoError(t, codec.Unmarshal(payload, &got))
	payload[0] = 9
	assert.Equal(t, []byte{1, 2, 3}, got, "unmarshal must copy")

	_, err = codec.Marshal("text")
	assert.Error(t, err)
	assert.Error(t, codec.Unmarshal(payload, new(string)))
}

func TestToStatusRoundTripsThroughFromStatus(t *testing.T) {
	for _, pe := range []*protocol.Error{
		{Code: protocol.CodeInvalidConfig, Message: "bad", Data: map[string]any{"field": "TOKEN"}},
		{Code: protocol.CodeUnsupported, Message: "unsupported method: dns01.check"},
		{Code: -31000, Message: "custom", Data: map[string]any{"value": "scalar"}},
	} {
		got := grpcbridge.FromStatus(context.Background(), grpcbridge.ToStatus(pe).Err(), true)
		assert.Equal(t, pe, got)
	}

	// A scalar data value is wrapped in an object.
	st := grpcbridge.ToStatus(&protocol.Error{Code: -31000, Message: "x", Data: 3})
	got := grpcbridge.FromStatus(context.Background(), st.Err(), true)
	assert.Equal(t, map[string]any{"value": float64(3)}, got.(*protocol.Error).Data)
}
