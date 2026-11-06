package protocol

// JSON-RPC 2.0 error codes used on the wire.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32000
	// CodePermissionDenied is returned when a plugin calls a host.* method
	// without having requested the matching permission in its manifest.
	CodePermissionDenied = -32001
	// CodeUnsupported is returned when the plugin does not implement a
	// capability method it is asked for.
	CodeUnsupported = -32002
	// CodeInvalidConfig is returned when credentials or settings fail
	// validation. Data.Field names the offending field.
	CodeInvalidConfig = -32003
)

// Error is the JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// InvalidConfigData is the conventional shape of Error.Data for CodeInvalidConfig.
type InvalidConfigData struct {
	Field string `json:"field,omitempty"`
}
