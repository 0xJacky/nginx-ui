---
outline: [2, 3]
---

# HTTP 接口

带有 `http` 能力的插件可以在 Nginx UI 之后提供 HTTP API、页面或 WebSocket。Nginx UI 先验证用户身份，再把 `/api/plugins/<插件 ID>/http/` 下的请求转发给插件。插件的浏览器包通过 `registry.http` 和 `registry.wsUrl` 访问这些接口，参见[浏览器包](./webapp.md#registry)。

```json [plugin.json]
"capabilities": ["http"],
"http": { "listen": "unix" }
```

`listen` 决定请求如何到达插件：

| 值 | 工作方式 | 适用于 |
| --- | --- | --- |
| `unix` | 插件运行自己的 HTTP 服务器，Nginx UI 代理到它。WebSocket 和流式响应会直接透传。 | 几乎所有场景。 |
| `rpc` | Nginx UI 把每个请求作为一次 `http.handle` 调用通过插件协议发送。不支持流式传输。 | 无法运行服务器的插件中的小型 API。 |

## 使用 SDK {#with-an-sdk}

两个 SDK 都提供 `unix` 方式，并替你完成本页其余的工作：打开监听、检查密钥、读取用户头和正常关闭。Go SDK 接受任意 `http.Handler`，Rust SDK 接受任意 `async fn(Request<Incoming>) -> Response<HttpBody>`：

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

在 Rust 中，axum 等路由器可以在这样的函数里用 `router.oneshot(req.map(Body::new))` 处理请求。本页其余部分面向不使用 SDK 编写的插件。

## 使用 `unix` 提供服务 {#serving-with-unix}

### 监听位置 {#where-to-listen}

- **Unix 套接字。** 在 Linux 和 macOS 上，插件精确地监听 `<NGINX_UI_PLUGIN_DATA_DIR>/http.sock`。监听前删除残留的套接字文件，以 `0600` 权限创建套接字，并在退出时删除它。当数据目录的路径对套接字路径而言过长时（macOS 上 104 字节，Linux 上 108 字节），插件无法提供服务，应以内部错误使握手失败并说明原因。
- **命名管道。** 在 Windows 上，插件以随机名称创建一个命名管道，例如 `\\.\pipe\nginx-ui-plugin-<随机值>`，并在[握手](./lifecycle.md#handshake)回复中以 `http_pipe` 报告它。创建第一个实例时，名称已存在则应失败；只允许插件运行所用的用户连接，并拒绝远程客户端。无法打开命名管道的插件改为在 `127.0.0.1` 上监听一个空闲端口，并报告 `http_port`。

监听必须在插件回复握手之前就能接受连接。Nginx UI 只接受本机上的管道，且名称以字母或数字开头、只包含字母、数字、点、连字符和下划线；其他名称会使握手失败。

### 校验密钥 {#checking-the-secret}

其他本地进程也可能连接到监听，所以 Nginx UI 会在每个请求上证明自己的身份：

- 每次启动时，Nginx UI 生成一个随机密钥，并通过环境变量 `NGINX_UI_PLUGIN_HTTP_SECRET` 传给插件。
- 它在转发的每个请求（包括 WebSocket 升级请求）的 `Nginx-UI-Plugin-Secret` 头中发送该密钥。
- 插件对没有恰好携带一个正确值的请求一律回复 `401`。请使用常量时间比较，绝不记录密钥，也不要把这个头传给处理程序。

在启动时读取一次该变量，然后把它从环境中删除，使子进程不会继承它。变量缺失时不要打开监听，而应以内部错误使握手失败并指出该变量。

### 调用者是谁 {#who-is-calling}

Nginx UI 会删除所有发给它自己的凭据，例如 `Authorization`、`Cookie` 以及集群中其他节点用于登录的头，还会删除客户端发送的所有 `Nginx-UI-*` 头，然后设置：

| 请求头 | 值 |
| --- | --- |
| `Nginx-UI-User-ID` | 已登录用户的 ID。 |
| `Nginx-UI-User` | 已登录用户的名称。 |

校验密钥之后，插件就可以信任这些请求头。每个用户可以做什么，仍由插件自己决定。

### 关闭 {#shutting-down}

收到 `plugin.shutdown` 时，停止接受新连接，让正在运行的请求在关闭时限内完成，然后关闭剩下的连接。WebSocket 这类长连接需要显式关闭。

## 使用 `rpc` 提供服务 {#serving-with-rpc}

每个请求都以一次 `http.handle` 调用到达：

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

| 字段 | 含义 |
| --- | --- |
| `method` | HTTP 方法。 |
| `path` | `/api/plugins/<插件 ID>/http` 之后的路径。 |
| `query` | 不带 `?` 的查询字符串。 |
| `headers` | 请求头，每个值都是字符串数组。 |
| `body_base64` | 请求体，base64 编码。 |
| `user` | 已登录的用户：`id` 和 `name`。 |

回复包含 `status`、`headers`（字符串数组）和 `body_base64`：

```json
{ "jsonrpc": "2.0", "id": 9, "result": { "status": 200, "headers": { "Content-Type": ["application/json"] }, "body_base64": "eyJvayI6dHJ1ZX0=" } }
```

整个请求和回复都在一条消息中传输，请保持较小的体积（单条消息上限 4 MiB）。
