package grpcbridge_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge/bridgetest"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	pluginv1 "github.com/0xJacky/Nginx-UI/internal/plugin/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// logSink records the entries of every stream and rejects status 500.
type logSink struct {
	mu      sync.Mutex
	entries []protocol.LogSinkPushParams
}

func (s *logSink) handle(_ context.Context, method string, next func() (json.RawMessage, error)) (any, error) {
	if method != protocol.MethodLogPush {
		return nil, &protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method: " + method}
	}
	var result protocol.LogSinkPushResult
	for {
		raw, err := next()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		var params protocol.LogSinkPushParams
		if err = json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		if params.Entry.Status == 500 {
			return nil, &protocol.Error{Code: protocol.CodeInternalError, Message: "destination down"}
		}
		s.mu.Lock()
		s.entries = append(s.entries, params)
		s.mu.Unlock()
		result.Accepted++
	}
}

func TestStreamingRPCsAreNotRoutedAsCalls(t *testing.T) {
	m, ok := grpcbridge.Lookup(protocol.MethodLogPush)
	require.True(t, ok)
	assert.True(t, m.Streaming)
	assert.Equal(t, "/nginxui.plugin.v1.LogSink/Push", m.FullMethod)
	assert.False(t, grpcbridge.IsCapability(protocol.MethodLogPush))
	assert.True(t, grpcbridge.IsStreaming(protocol.MethodLogPush))
	assert.False(t, grpcbridge.IsStreaming(protocol.MethodDNS01Present))

	client := startServer(t, bridgetest.NewServer(newFakePlugin().handle), grpcbridge.Endpoint{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.ErrorIs(t, client.Call(ctx, protocol.MethodLogPush, nil, nil), grpcbridge.ErrStreamingMethod)
	_, err := client.OpenStream(ctx, protocol.MethodDNS01Present)
	assert.ErrorIs(t, err, grpcbridge.ErrStreamingMethod)
	_, err = client.OpenStream(ctx, "log.tail")
	assert.ErrorIs(t, err, grpcbridge.ErrNotInContract)
}

func TestClientStreamRoundTrip(t *testing.T) {
	sink := &logSink{}
	client := startServer(t, bridgetest.NewStreamServer(newFakePlugin().handle, sink.handle), grpcbridge.Endpoint{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := client.OpenStream(ctx, protocol.MethodLogPush)
	require.NoError(t, err)
	// Generated messages and JSON values travel the same way.
	in, err := proto.Marshal(&pluginv1.LogSinkPushRequest{
		LogPath: "/var/log/nginx/access.log",
		Entry:   &pluginv1.LogEntry{Status: 200, BodyBytesSent: float64(5 << 30), Format: protocol.LogFormatCombined},
	})
	require.NoError(t, err)
	require.NoError(t, stream.Send(in))
	require.NoError(t, stream.SendJSON(protocol.LogSinkPushParams{
		LogPath: "/var/log/nginx/other.log",
		Entry:   protocol.LogEntry{Status: 404, Raw: "x", Format: protocol.LogFormatRaw},
	}))
	out, err := stream.CloseAndRecv()
	require.NoError(t, err)

	var res pluginv1.LogSinkPushResponse
	require.NoError(t, proto.Unmarshal(out, &res))
	assert.EqualValues(t, 2, res.GetAccepted())
	require.Len(t, sink.entries, 2)
	assert.Equal(t, protocol.ByteSize(5<<30), sink.entries[0].Entry.BodyBytesSent)
	assert.Equal(t, "/var/log/nginx/other.log", sink.entries[1].LogPath)

	// A failing stream surfaces the plugin error like a call does.
	stream, err = client.OpenStream(ctx, protocol.MethodLogPush)
	require.NoError(t, err)
	require.NoError(t, stream.SendJSON(protocol.LogSinkPushParams{Entry: protocol.LogEntry{Status: 500}}))
	_, err = stream.CloseAndRecv()
	var perr *protocol.Error
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, protocol.CodeInternalError, perr.Code)
	assert.Equal(t, "destination down", perr.Message)

	// A server without the stream answers method not found.
	plain := startServer(t, bridgetest.NewServer(newFakePlugin().handle), grpcbridge.Endpoint{})
	stream, err = plain.OpenStream(ctx, protocol.MethodLogPush)
	require.NoError(t, err)
	_, err = stream.CloseAndRecv()
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, protocol.CodeMethodNotFound, perr.Code)
}
