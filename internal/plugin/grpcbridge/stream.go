package grpcbridge

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

// ClientStream is an open client stream of a streaming rpc (spec WIRE-12):
// the caller sends any number of request messages, closes the stream and
// receives one response. It is not safe for concurrent use.
type ClientStream struct {
	ctx    context.Context
	method *Method
	stream grpc.ClientStream
}

// OpenStream opens a client stream of a streaming rpc of the contract. ctx
// bounds the whole stream, cancelling it aborts the stream.
func (c *Client) OpenStream(ctx context.Context, method string) (*ClientStream, error) {
	m, ok := Lookup(method)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotInContract, method)
	}
	if !m.Streaming {
		return nil, fmt.Errorf("%w: %s", ErrStreamingMethod, method)
	}
	desc := &grpc.StreamDesc{StreamName: m.FullMethod, ClientStreams: true}
	stream, err := c.conn.NewStream(ctx, desc, m.FullMethod)
	if err != nil {
		return nil, FromStatus(ctx, err, true)
	}
	return &ClientStream{ctx: ctx, method: m, stream: stream}, nil
}

// Method is the rpc of the stream.
func (s *ClientStream) Method() *Method { return s.method }

// Send sends one request message in protobuf encoding. An error ends the
// stream; CloseAndRecv then reports why the peer ended it.
func (s *ClientStream) Send(in []byte) error {
	return s.stream.SendMsg(&in)
}

// SendJSON converts a JSON value into the request message and sends it.
func (s *ClientStream) SendJSON(value any) error {
	in, err := EncodeJSON(s.method.Input, value)
	if err != nil {
		return err
	}
	return s.Send(in)
}

// CloseAndRecv closes the sending side and waits for the response message in
// protobuf encoding. Errors map like the ones of Client.Call.
func (s *ClientStream) CloseAndRecv() ([]byte, error) {
	if err := s.stream.CloseSend(); err != nil {
		return nil, FromStatus(s.ctx, err, true)
	}
	var out []byte
	if err := s.stream.RecvMsg(&out); err != nil {
		return nil, FromStatus(s.ctx, err, true)
	}
	return out, nil
}
