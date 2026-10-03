---
outline: [2, 3]
---

# Protocol

Nginx UI and a plugin process exchange [JSON-RPC 2.0](https://www.jsonrpc.org/specification)
messages over the process's standard input and output. Both sides send
requests: Nginx UI calls the lifecycle and capability methods, and the plugin
calls the host API. A plugin may also serve its capability calls over gRPC,
which is faster for large payloads and required for access log streaming.

::: tip
The SDKs implement everything on this page. Read it when you write a plugin
without an SDK, or when you want to know what happens underneath.
:::

## Framing

- Nginx UI writes to the plugin's standard input; the plugin writes to its
  standard output.
- Every message is one JSON object on one line, followed by a line feed. A
  message never contains a raw line break.
- **Standard output carries protocol messages only.** Write logs, banners and
  stack traces to standard error, which Nginx UI captures for the plugin log
  and never parses.
- Batches (a JSON array of messages) are not supported.
- A message is at most **4 MiB** including its line feed. A larger message
  ends the connection.

## Messages

Every message has `"jsonrpc": "2.0"`. The other members decide its kind:

| Kind | `id` | `method` | `params` | `result` | `error` |
| --- | --- | --- | --- | --- | --- |
| Request | present, not `null` | present | optional | | |
| Notification | absent | present | optional | | |
| Success response | the request's `id` | | | present | |
| Error response | the request's `id` | | | | present |

- A request gets exactly one response. A notification never gets one.
- `id` is an opaque value for matching a response to its request. The SDKs use
  small integers.
- `params` is always an object.

Requests may run concurrently and be answered in any order, so match responses
by `id`. A plugin may handle requests concurrently or one at a time; it must
still answer health checks while busy (see [Lifecycle](./lifecycle.md#health-checks)).

## Errors

```json
{ "jsonrpc": "2.0", "id": 7, "error": { "code": -32003, "message": "the API token is required", "data": { "field": "MYDNS_API_TOKEN" } } }
```

`message` is short, readable and never contains a credential. `data` is an
optional object with details.

### Error Codes

| Code | Name | Use it when |
| --- | --- | --- |
| `-32700` | Parse error | The message is not valid JSON. |
| `-32600` | Invalid request | The message is JSON but not a valid JSON-RPC message. |
| `-32601` | Method not found | No handler exists for the method. |
| `-32602` | Invalid params | `params` do not have the expected shape, or name something the method does not know where a capability page says so. |
| `-32000` | Internal error | The call ran and failed for any other reason: a vendor outage, a network failure. |
| `-32001` | Permission denied | A host API call the plugin's permissions do not cover. |
| `-32002` | Unsupported | An optional method of a declared capability that the plugin does not implement. |
| `-32003` | Invalid config | A value the person entered is missing or wrong. Set `data.field` to the key of the field. |

::: tip
The difference between `-32003` and `-32000` matters: Nginx UI shows `-32003`
next to the offending field so the person can fix it, and treats `-32000` as a
failure of the service. Do not invent other codes in the range `-32000` to
`-32099`.
:::

## JSON Shapes

Every `params` and `result` is the
[protobuf JSON mapping](https://protobuf.dev/programming-guides/json/) of a
message defined in the [proto files](https://github.com/nginxui/plugin-spec/tree/main/proto/nginxui/plugin/v1)
of the plugin-spec repository. The pages of this guide show every message as
JSON, so you need the proto files only to generate code.

- Member names use `snake_case`, such as `effective_fqdn`, never `camelCase`.
- An absent member and a member with its default value (`""`, `0`, `false`,
  `[]`, `{}`, `null`) mean the same. Senders may leave defaults out; receivers
  treat a missing member as its default.
- Numbers are JSON numbers. Byte sizes that may exceed 4 GiB are doubles,
  exact up to 2^53; accept `5242880` and `5242880.0` alike.
- Binary data is base64 with padding.
- A method without parameters may omit `params`. A method without a result
  answers `{}`.
- **Ignore members you do not know.** New optional members are how the
  protocol grows without breaking older plugins.

## Connection Lifetime

The connection is the process's standard input and output. It ends when the
process exits, when either side closes its end, or when a message is too
large. A new process means a new connection and a new handshake.

## gRPC Transport

A plugin may additionally serve its capability calls over
[gRPC](https://grpc.io). Methods, messages, errors and results stay the same;
only the transport changes. Nginx UI uses it when it can and falls back to
standard input and output otherwise, so a plugin must keep serving every
method there as well.

### Opting In

The plugin lists `grpc` in the `transports` of its handshake answer and says
where it listens:

```json
{
  "jsonrpc": "2.0", "id": 1,
  "result": {
    "api_version": 1,
    "capabilities": ["dns01"],
    "transports": ["stdio", "grpc"],
    "rpc_socket": "/var/lib/nginx-ui/plugins/.data/com.example.mydns/rpc.sock"
  }
}
```

- **Unix socket.** By default the plugin listens on
  `<NGINX_UI_PLUGIN_DATA_DIR>/rpc.sock`. A socket path is limited to 104 bytes
  on macOS and the BSDs and 108 on Linux; when the default does not fit, the
  plugin listens in a private directory (mode `0700`) under the system
  temporary directory and reports the path in `rpc_socket`. Remove a stale
  socket before listening, create the socket with mode `0600` and remove it on
  exit.
- **Named pipe.** On Windows the plugin creates a named pipe under a random
  name and reports it in `rpc_pipe`, together with `rpc_token`, a random value
  of at least 128 bits. The pipe follows the same rules as the
  [HTTP pipe](./http.md#where-to-listen). Nginx UI sends
  `authorization: Bearer <rpc_token>` with every call, and the plugin rejects
  calls without exactly that value as `UNAUTHENTICATED`.
- **Loopback TCP.** A Windows plugin that cannot open a named pipe listens on
  `127.0.0.1` and reports `rpc_port` and `rpc_token` instead, with the same
  token check.

The listener must accept connections before the plugin answers the
handshake. Nginx UI then checks the channel with `plugin.ping` over gRPC; when
the plugin does not list `grpc`, or the check fails, it uses standard input
and output for the life of the process.

### What Travels Where

| Traffic | Transport |
| --- | --- |
| The handshake, `plugin.initialized`, `plugin.shutdown`, `plugin.exit` | Standard input and output, always |
| `plugin.configure` and the health checks | Standard input and output |
| Host API calls and their answers | Standard input and output |
| Events and scheduled calls | Standard input and output |
| Capability calls (`dns01.*`, `http.handle`, `notify.*`, `probe.check`, `mcp.call`, `storage.*`, `deploy.*`, `blocklist.fetch`, `discovery.resolve`) | gRPC while the channel works, otherwise standard input and output |
| Streams (`log.push`) | gRPC only |

A plugin serving gRPC must serve every capability method and `plugin.ping`
there, and must return the same result or error on both transports. It
ignores the handshake and stop methods if they arrive over gRPC.

When the channel breaks while the process runs, Nginx UI switches back to
standard input and output for the rest of the process's life. A call that
fails with `UNAVAILABLE` and no plugin error detail is retried once there, so
do not answer `UNAVAILABLE` yourself.

### gRPC Messages and Errors

The gRPC service paths come from the proto files, for example
`/nginxui.plugin.v1.DNS01/Present`, and the messages use the standard protobuf
encoding, so generated stubs work. Messages of up to 64 MiB are accepted in
either direction. Nginx UI passes its deadline as the gRPC deadline; stop
working on a call once it expires.

A failed call carries a gRPC status and, as a status detail, a
`nginxui.plugin.v1.PluginError` with the JSON-RPC `code` and `data`:

| JSON-RPC code | gRPC status |
| --- | --- |
| `-32601` Method not found, `-32002` Unsupported | `UNIMPLEMENTED` |
| `-32602` Invalid params, `-32003` Invalid config | `INVALID_ARGUMENT` |
| `-32001` Permission denied | `PERMISSION_DENIED` |
| `-32000` Internal error | `INTERNAL` |
| Any other code | `UNKNOWN` |

A receiver uses the detail's code when present and maps the status back
otherwise.

### Streams

Some traffic is a flow rather than a call. The only one today is `log.push`,
which hands a plugin the access log lines (see
[Access Log Streaming](./capabilities/log-sink.md)). It is a client streaming
gRPC call: Nginx UI sends any number of messages, closes its side and the
plugin answers once. A stream has no JSON-RPC form: it never travels over
standard input and output, where its method name gets `-32601` like any
unknown method.
