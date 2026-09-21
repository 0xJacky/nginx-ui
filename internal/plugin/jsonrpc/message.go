package jsonrpc

import (
	"encoding/json"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// Version is the only JSON-RPC version accepted on the wire.
const Version = "2.0"

// message is the union of every frame shape the peer can send. JSON-RPC 2.0
// keeps requests, notifications and responses distinguishable by which members
// are present, so one struct can decode all of them.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *protocol.Error `json:"error,omitempty"`
}

// isRequest reports whether the frame expects a response.
func (m *message) isRequest() bool {
	return m.Method != "" && len(m.ID) > 0 && !isNullJSON(m.ID)
}

// isNotification reports whether the frame must not be answered.
func (m *message) isNotification() bool {
	return m.Method != "" && (len(m.ID) == 0 || isNullJSON(m.ID))
}

// isResponse reports whether the frame answers a call we made.
func (m *message) isResponse() bool {
	return m.Method == "" && len(m.ID) > 0
}

// Notification is one entry of a NotifyBatch call.
type Notification struct {
	Method string
	Params any
}

// outgoingRequest is the frame written by Call.
type outgoingRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// outgoingNotification is the frame written by Notify.
type outgoingNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// outgoingResponse is the frame written when a Handler returns.
type outgoingResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *protocol.Error `json:"error,omitempty"`
}

// nullID is the id used for replies that cannot be attributed to a request.
var nullID = json.RawMessage("null")

func isNullJSON(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case 'n':
			return string(trimSpace(raw)) == "null"
		default:
			return false
		}
	}
	return true
}

func trimSpace(raw json.RawMessage) json.RawMessage {
	start := 0
	for start < len(raw) && isSpace(raw[start]) {
		start++
	}
	end := len(raw)
	for end > start && isSpace(raw[end-1]) {
		end--
	}
	return raw[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}
