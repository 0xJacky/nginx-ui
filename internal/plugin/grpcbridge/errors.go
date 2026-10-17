package grpcbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	pluginv1 "github.com/0xJacky/Nginx-UI/internal/plugin/protocol/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// ErrUnavailable reports that the gRPC channel itself failed, as opposed to
// the plugin answering with an error. The caller may retry on stdio.
var ErrUnavailable = errors.New("grpcbridge: transport unavailable")

// ErrNotInContract is returned for a method that is no rpc of the contract
// and therefore has no gRPC path.
var ErrNotInContract = errors.New("grpcbridge: method is not an rpc of the plugin contract")

// ErrStreamingMethod is returned when a streaming rpc is used as a unary call
// or a unary rpc is opened as a stream.
var ErrStreamingMethod = errors.New("grpcbridge: streaming and unary rpcs are not interchangeable")

// FromStatus maps the error of a gRPC call back onto what the same call
// returns on stdio: the PluginError detail when present, otherwise
// the status code table. The caller's context wins, so a deadline or a
// cancellation surfaces as the context error exactly as on stdio. inContract
// tells whether the called path is an rpc of the contract, which decides the
// meaning of a bare UNIMPLEMENTED.
func FromStatus(ctx context.Context, err error, inContract bool) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	for _, detail := range st.Details() {
		if pe, ok := detail.(*pluginv1.PluginError); ok {
			message := pe.GetMessage()
			if message == "" {
				message = st.Message()
			}
			return &protocol.Error{Code: int(pe.GetCode()), Message: message, Data: errorData(pe.GetData())}
		}
	}

	var code int
	switch st.Code() {
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.Canceled:
		return context.Canceled
	case codes.Unavailable:
		return fmt.Errorf("%w: %s", ErrUnavailable, st.Message())
	case codes.InvalidArgument:
		code = protocol.CodeInvalidParams
	case codes.Unimplemented:
		code = protocol.CodeMethodNotFound
		if inContract {
			code = protocol.CodeUnsupported
		}
	case codes.PermissionDenied, codes.Unauthenticated:
		code = protocol.CodePermissionDenied
	default:
		code = protocol.CodeInternalError
	}
	return &protocol.Error{Code: code, Message: st.Message()}
}

// errorData returns the detail data the way encoding/json decodes the stdio
// error object, so both transports hand the caller the same value.
func errorData(data *structpb.Struct) any {
	if data == nil {
		return nil
	}
	return data.AsMap()
}

// CodeFor maps a JSON-RPC error code onto a gRPC status code.
func CodeFor(code int) codes.Code {
	switch code {
	case protocol.CodeParseError, protocol.CodeInvalidRequest, protocol.CodeInvalidParams, protocol.CodeInvalidConfig:
		return codes.InvalidArgument
	case protocol.CodeMethodNotFound, protocol.CodeUnsupported:
		return codes.Unimplemented
	case protocol.CodeInternalError:
		return codes.Internal
	case protocol.CodePermissionDenied:
		return codes.PermissionDenied
	default:
		return codes.Unknown
	}
}

// ToStatus builds the gRPC status a server returns for a JSON-RPC error: the
// mapped code, the message, and the error itself as a PluginError detail.
func ToStatus(pe *protocol.Error) *status.Status {
	st := status.New(CodeFor(pe.Code), pe.Message)
	detail := &pluginv1.PluginError{Code: int32(pe.Code), Message: pe.Message, Data: structData(pe.Data)}
	if withDetail, err := st.WithDetails(detail); err == nil {
		return withDetail
	}
	return st
}

// structData converts error data to a Struct, wrapping a value that is not a
// JSON object as {"value": data}.
func structData(data any) *structpb.Struct {
	if data == nil {
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil || isNullJSON(raw) {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") {
		out := &structpb.Struct{}
		if protojson.Unmarshal(raw, out) != nil {
			return nil
		}
		return out
	}
	value := &structpb.Value{}
	if protojson.Unmarshal(raw, value) != nil {
		return nil
	}
	return &structpb.Struct{Fields: map[string]*structpb.Value{"value": value}}
}
