// Package protocol defines the wire contract between nginx-ui and plugin
// processes. Every type here maps one to one onto a JSON-RPC 2.0 method
// exchanged as NDJSON over the plugin's stdin and stdout.
//
// The source of truth is the proto contract in the nginx-ui-plugin-spec
// repository (proto/nginxui/plugin/v1). The JSON form of a message is its
// protobuf JSON mapping with proto field names, so every json tag here equals
// a proto field name. Package pb holds a verbatim copy of the generated Go
// code, and alignment_test.go fails when a type here drifts from its message.
// Change the proto first, then run pb/regen.sh and update these types.
//
// The plugin SDK ships an identical copy of these types so that plugins do not
// depend on the nginx-ui module tree. Keep the two in sync: only add fields,
// never rename or change the JSON tag of an existing one.
package protocol

// APIVersion is the protocol major version implemented by this host.
const APIVersion = 1
