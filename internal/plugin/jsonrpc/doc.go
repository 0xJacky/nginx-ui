// Package jsonrpc implements the bidirectional JSON-RPC 2.0 peer used to talk
// to plugin processes. Messages are newline delimited JSON: exactly one JSON
// value per line, in both directions.
//
// The package is transport agnostic. A Conn is built from any io.Reader and
// io.Writer pair, which lets the supervisor wire it to a child process' stdout
// and stdin while tests wire two Conns to each other through in-memory pipes.
package jsonrpc
