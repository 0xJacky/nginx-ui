package grpcbridge

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// RawCodec passes message bytes through untouched, so the bridge can encode
// and decode with dynamic messages instead of generated types. The bytes are
// protobuf, so the codec keeps the standard "proto" content subtype and a
// peer with generated stubs sees an ordinary call.
type RawCodec struct{}

// Name implements encoding.Codec.
func (RawCodec) Name() string { return "proto" }

// Marshal implements encoding.Codec for []byte and *[]byte.
func (RawCodec) Marshal(v any) ([]byte, error) {
	switch m := v.(type) {
	case []byte:
		return m, nil
	case *[]byte:
		return *m, nil
	default:
		return nil, fmt.Errorf("grpcbridge: raw codec cannot marshal %T", v)
	}
}

// Unmarshal implements encoding.Codec for *[]byte. The bytes are copied since
// the transport reuses its buffer.
func (RawCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("grpcbridge: raw codec cannot unmarshal into %T", v)
	}
	*p = append((*p)[:0], data...)
	return nil
}

// isNullJSON reports whether raw carries no value.
func isNullJSON(raw []byte) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

// EncodeJSON converts a JSON value into the protobuf bytes of md. Members the
// message does not define are ignored.
func EncodeJSON(md protoreflect.MessageDescriptor, value any) ([]byte, error) {
	msg := dynamicpb.NewMessage(md)
	if value != nil {
		raw, ok := value.(json.RawMessage)
		if !ok {
			var err error
			if raw, err = json.Marshal(value); err != nil {
				return nil, err
			}
		}
		if !isNullJSON(raw) {
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, msg); err != nil {
				return nil, err
			}
		}
	}
	return proto.Marshal(msg)
}

// DecodeJSON converts the protobuf bytes of md into JSON with proto field
// names, the form the JSON-RPC side of the contract uses.
func DecodeJSON(md protoreflect.MessageDescriptor, data []byte) (json.RawMessage, error) {
	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
}
