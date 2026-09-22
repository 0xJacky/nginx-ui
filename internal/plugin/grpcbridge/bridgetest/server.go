// Package bridgetest serves the plugin contract over gRPC from a JSON-RPC
// style handler, the way the Go SDK does. It exists for tests that need a
// plugin speaking gRPC without a real plugin binary.
package bridgetest

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/0xJacky/Nginx-UI/internal/plugin/grpcbridge"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"google.golang.org/grpc"
)

// Handler answers one rpc by its JSON-RPC name. Returning a *protocol.Error
// sends that error as the PluginError detail; any other error becomes -32000.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// NewServer returns a gRPC server that resolves every path through the
// contract descriptors and answers with h. Paths outside the contract answer
// UNIMPLEMENTED with a -32601 detail, undecodable requests INVALID_ARGUMENT
// with -32602.
func NewServer(h Handler, opts ...grpc.ServerOption) *grpc.Server {
	base := []grpc.ServerOption{
		grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			return serve(stream, h)
		}),
		grpc.ForceServerCodec(grpcbridge.RawCodec{}),
	}
	return grpc.NewServer(append(base, opts...)...)
}

func serve(stream grpc.ServerStream, h Handler) error {
	fullMethod, _ := grpc.MethodFromServerStream(stream)
	var in []byte
	if err := stream.RecvMsg(&in); err != nil {
		return err
	}

	m, ok := grpcbridge.LookupFullMethod(fullMethod)
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
