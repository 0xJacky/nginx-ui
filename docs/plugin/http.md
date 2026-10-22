---
outline: [2, 3]
---

# HTTP Endpoints

A plugin with the `http` capability serves an HTTP API, pages or WebSockets
behind Nginx UI. Nginx UI authenticates the person, then forwards requests
under `/api/plugins/<plugin id>/http/` to the plugin. The browser bundle of the
plugin reaches them through `registry.http` and `registry.wsUrl`, see
[Browser Bundle](./webapp.md#registry).

```json [plugin.json]
"capabilities": ["http"],
"http": { "listen": "unix" }
```

`listen` chooses how requests reach the plugin:

| Value | How it works | Use it for |
| --- | --- | --- |
| `unix` | The plugin runs an HTTP server of its own, and Nginx UI proxies to it. WebSockets and streamed responses pass through. | Almost everything. |
| `rpc` | Nginx UI sends every request as one `http.handle` call over the plugin protocol. No streaming. | Small APIs in plugins that cannot run a server. |

## With an SDK

Both SDKs serve `unix` and do the rest of this page for you: they open the
listener, check the secret, read the user headers and shut down cleanly. Give
the Go SDK any `http.Handler`, and the Rust SDK any
`async fn(Request<Incoming>) -> Response<HttpBody>`:

::: code-group

```go [Go]
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		user := sdk.UserFromRequest(r)
		fmt.Fprintf(w, "hello %s", user.Name)
	})

	sdk.Serve(sdk.Plugin{HTTP: mux})
}
```

```rust [Rust]
use nginxui_plugin_sdk::http::{
    text_response, user_from_request, HttpBody, Incoming, Request, Response, StatusCode,
};
use nginxui_plugin_sdk::Plugin;

async fn hello(req: Request<Incoming>) -> Response<HttpBody> {
    let user = user_from_request(&req);
    text_response(StatusCode::OK, format!("hello {}", user.name))
}

fn main() {
    nginxui_plugin_sdk::serve_blocking(Plugin::new().http(hello));
}
```

:::

In Rust, a router such as axum serves a request with
`router.oneshot(req.map(Body::new))` inside such a function. The rest of this
page is for plugins written without an SDK.

## Serving with `unix`

### Where to Listen

- **Unix socket.** On Linux and macOS the plugin listens on
  `<NGINX_UI_PLUGIN_DATA_DIR>/http.sock`, exactly that path. Remove a stale
  socket file before listening, create the socket with mode `0600` and remove
  it on exit. When the data directory path is too long for a socket path (104
  bytes on macOS, 108 on Linux), the plugin cannot serve and should fail the
  handshake with an internal error that says why.
- **Named pipe.** On Windows the plugin creates a named pipe under a random
  name, such as `\\.\pipe\nginx-ui-plugin-<random>`, and reports it as
  `http_pipe` in its [handshake](./lifecycle.md#handshake) answer. Create the
  first instance so that it fails when the name exists, allow only the user
  the plugin runs as and refuse remote clients. A plugin that cannot open a
  named pipe listens on `127.0.0.1` with a free port and reports `http_port`
  instead.

The listener must accept connections before the plugin answers the handshake.
Nginx UI accepts only a pipe on the local machine whose name starts with a
letter or digit and uses letters, digits, dots, dashes and underscores; any
other name fails the handshake.

### Checking the Secret

Other local processes may reach the listener, so Nginx UI proves itself on
every request:

- At each start Nginx UI generates a random secret and passes it in the
  environment variable `NGINX_UI_PLUGIN_HTTP_SECRET`.
- It sends the secret in the `Nginx-UI-Plugin-Secret` header of every
  request it forwards, WebSocket upgrades included.
- The plugin answers `401` to every request that does not carry exactly one
  such header with the right value. Compare in constant time, never log the
  secret and do not pass the header to your handlers.

Read the variable once at start and remove it from the environment, so child
processes do not inherit it. When the variable is missing, do not open the
listener; fail the handshake with an internal error that names the variable.

### Who Is Calling

Nginx UI removes every credential meant for itself, such as `Authorization`,
`Cookie` and the headers another node of a cluster signs in with, and every
`Nginx-UI-*` header the client sent. It then sets:

| Header | Value |
| --- | --- |
| `Nginx-UI-User-ID` | Id of the signed in person. |
| `Nginx-UI-User` | Name of the signed in person. |

After checking the secret, a plugin can trust these headers. It still decides
itself what each person may do.

### Shutting Down

On `plugin.shutdown` stop accepting connections, let running requests finish
within the shutdown time and close the rest. Long lived connections such as
WebSockets need closing explicitly.

## Serving with `rpc`

Each request arrives as an `http.handle` call:

```json
{
  "jsonrpc": "2.0", "id": 9, "method": "http.handle",
  "params": {
    "method": "GET",
    "path": "/status",
    "query": "verbose=1",
    "headers": { "Accept": ["application/json"] },
    "body_base64": "",
    "user": { "id": "1", "name": "admin" }
  }
}
```

| Field | Meaning |
| --- | --- |
| `method` | HTTP method. |
| `path` | Path below `/api/plugins/<plugin id>/http`. |
| `query` | Query string without the `?`. |
| `headers` | Request headers, each an array of strings. |
| `body_base64` | Request body, base64. |
| `user` | The signed in person: `id` and `name`. |

The answer carries `status`, `headers` (arrays of strings) and `body_base64`:

```json
{ "jsonrpc": "2.0", "id": 9, "result": { "status": 200, "headers": { "Content-Type": ["application/json"] }, "body_base64": "eyJvayI6dHJ1ZX0=" } }
```

The whole request and answer travel in one message, so keep them small (a
message is limited to 4 MiB).
