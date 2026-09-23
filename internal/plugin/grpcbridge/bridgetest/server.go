// Package bridgetest serves the plugin contract over gRPC from a JSON-RPC
// style handler, the way the Go SDK does. It exists for tests that need a
// plugin speaking gRPC without a real plugin binary.
package bridgetest

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"google.golang.org/grpc"
)

// Handler answers one rpc by its JSON-RPC name. Returning a *protocol.Error
// sends that error as the PluginError detail; any other error becomes -32000.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// StreamHandler answers one client stream of a streaming rpc by its JSON-RPC
// name. next returns every request message as JSON and io.EOF once the
// caller closed the stream.
type StreamHandler func(ctx context.Context, method string, next func() (json.RawMessage, error)) (any, error)

// NewServer returns a gRPC server that resolves every path through the
// contract descriptors and answers with h. Paths outside the contract answer
// UNIMPLEMENTED with a -32601 detail, undecodable requests INVALID_ARGUMENT
// with -32602. Streaming rpcs answer UNIMPLEMENTED, see NewStreamServer.
func NewServer(h Handler, opts ...grpc.ServerOption) *grpc.Server {
	return NewStreamServer(h, nil, opts...)
}

// NewStreamServer is NewServer that also serves the streaming rpcs with sh.
func NewStreamServer(h Handler, sh StreamHandler, opts ...grpc.ServerOption) *grpc.Server {
	base := []grpc.ServerOption{
		grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			return serve(stream, h, sh)
		}),
		grpc.ForceServerCodec(grpcbridge.RawCodec{}),
	}
	return grpc.NewServer(append(base, opts...)...)
}

func serve(stream grpc.ServerStream, h Handler, sh StreamHandler) error {
	fullMethod, _ := grpc.MethodFromServerStream(stream)
	m, ok := grpcbridge.LookupFullMethod(fullMethod)
	if ok && m.Streaming {
		return serveStream(stream, m, sh)
	}

	var in []byte
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}
	if !ok {
		return grpcbridge.ToStatus(&protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method: " + fullMethod}).Err()
	}

	params, err := grpcbridge.DecodeJSON(m.Input, in)
	if err != nil {
		return grpcbridge.ToStatus(&protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}).Err()
	}

	result, err := h(stream.Context(), m.RPCName, params)
	if err != nil {
		var pe *protocol.Error
		if !errors.As(err, &pe) {
			pe = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
		}
		return grpcbridge.ToStatus(pe).Err()
	}

	out, err := grpcbridge.EncodeJSON(m.Output, result)
	if err != nil {
		return grpcbridge.ToStatus(&protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}).Err()
	}
	return stream.SendMsg(out)
}

func serveStream(stream grpc.ServerStream, m *grpcbridge.Method, sh StreamHandler) error {
	if sh == nil {
		return grpcbridge.ToStatus(&protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown method: " + m.RPCName}).Err()
	}
	next := func() (json.RawMessage, error) {
		var in []byte
		if err := stream.RecvMsg(&in); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			return nil, err
		}
		params, err := grpcbridge.DecodeJSON(m.Input, in)
		if err != nil {
			return nil, &protocol.Error{Code: protocol.CodeInvalidParams, Message: err.Error()}
		}
		return params, nil
	}

	result, err := sh(stream.Context(), m.RPCName, next)
	if err != nil {
		var pe *protocol.Error
		if !errors.As(err, &pe) {
			pe = &protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}
		}
		return grpcbridge.ToStatus(pe).Err()
	}
	out, err := grpcbridge.EncodeJSON(m.Output, result)
	if err != nil {
		return grpcbridge.ToStatus(&protocol.Error{Code: protocol.CodeInternalError, Message: err.Error()}).Err()
	}
	return stream.SendMsg(out)
}
