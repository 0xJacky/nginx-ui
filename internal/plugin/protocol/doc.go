// Package protocol defines the wire contract between nginx-ui and plugin
// processes. Every type here maps one to one onto a JSON-RPC 2.0 method
// exchanged as NDJSON over the plugin's stdin and stdout.
//
// The plugin SDK ships an identical copy of these types so that plugins do not
// depend on the nginx-ui module tree. Keep the two in sync: only add fields,
// never rename or change the JSON tag of an existing one.
package protocol

// APIVersion is the protocol major version implemented by this host.
const APIVersion = 1
