---
outline: [2, 3]
---

# HTTP 介面

帶有 `http` 能力的外掛可以在 Nginx UI 之後提供 HTTP API、頁面或 WebSocket。Nginx UI 先驗證使用者身分，再把 `/api/plugins/<外掛 ID>/http/` 下的請求轉發給外掛。外掛的瀏覽器套件透過 `registry.http` 和 `registry.wsUrl` 存取這些介面，參見[瀏覽器套件](./webapp.md#registry)。

```json [plugin.json]
"capabilities": ["http"],
"http": { "listen": "unix" }
```

`listen` 決定請求如何到達外掛：

| 值 | 運作方式 | 適用於 |
| --- | --- | --- |
| `unix` | 外掛執行自己的 HTTP 伺服器，Nginx UI 代理到它。WebSocket 和串流回應會直接透傳。 | 幾乎所有情境。 |
| `rpc` | Nginx UI 把每個請求作為一次 `http.handle` 呼叫透過外掛協定傳送。不支援串流。 | 無法執行伺服器的外掛中的小型 API。 |

## 使用 SDK {#with-an-sdk}

兩個 SDK 都提供 `unix` 方式，並替你完成本頁其餘的工作：開啟監聽、檢查密鑰、讀取使用者標頭和正常關閉。Go SDK 接受任意 `http.Handler`，Rust SDK 接受任意 `async fn(Request<Incoming>) -> Response<HttpBody>`：

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

在 Rust 中，axum 等路由器可以在這樣的函式裡用 `router.oneshot(req.map(Body::new))` 處理請求。本頁其餘部分適用於不使用 SDK 撰寫的外掛。

## 使用 `unix` 提供服務 {#serving-with-unix}

### 監聽位置 {#where-to-listen}

- **Unix 套接字。** 在 Linux 和 macOS 上，外掛精確地監聽 `<NGINX_UI_PLUGIN_DATA_DIR>/http.sock`。監聽前刪除殘留的套接字檔案，以 `0600` 權限建立套接字，並在結束時刪除它。當資料目錄的路徑對套接字路徑而言過長時（macOS 上 104 位元組，Linux 上 108 位元組），外掛無法提供服務，應以內部錯誤使交握失敗並說明原因。
- **具名管道。** 在 Windows 上，外掛以隨機名稱建立一個具名管道，例如 `\\.\pipe\nginx-ui-plugin-<隨機值>`，並在[交握](./lifecycle.md#handshake)回覆中以 `http_pipe` 回報它。建立第一個執行個體時，名稱已存在則應失敗；只允許外掛執行所用的使用者連線，並拒絕遠端用戶端。無法開啟具名管道的外掛改為在 `127.0.0.1` 上監聽一個空閒連接埠，並回報 `http_port`。

監聽必須在外掛回覆交握之前就能接受連線。Nginx UI 只接受本機上的管道，且名稱以字母或數字開頭、只包含字母、數字、點、連字號和底線；其他名稱會使交握失敗。

### 驗證密鑰 {#checking-the-secret}

其他本機進程也可能連線到監聽，所以 Nginx UI 會在每個請求上證明自己的身分：

- 每次啟動時，Nginx UI 產生一個隨機密鑰，並透過環境變數 `NGINX_UI_PLUGIN_HTTP_SECRET` 傳給外掛。
- 它在轉發的每個請求（包括 WebSocket 升級請求）的 `Nginx-UI-Plugin-Secret` 標頭中傳送該密鑰。
- 外掛對沒有恰好攜帶一個正確值的請求一律回覆 `401`。請使用常數時間比較，絕不記錄密鑰，也不要把這個標頭傳給處理程式。

在啟動時讀取一次該變數，然後把它從環境中刪除，使子進程不會繼承它。變數缺少時不要開啟監聽，而應以內部錯誤使交握失敗並指出該變數。

### 呼叫者是誰 {#who-is-calling}

Nginx UI 會刪除所有發給它自己的認證，例如 `Authorization`、`Cookie` 以及叢集中其他節點用於登入的標頭，還會刪除用戶端傳送的所有 `Nginx-UI-*` 標頭，然後設定：

| 標頭 | 值 |
| --- | --- |
| `Nginx-UI-User-ID` | 已登入使用者的 ID。 |
| `Nginx-UI-User` | 已登入使用者的名稱。 |

驗證密鑰之後，外掛就可以信任這些標頭。每個使用者可以做什麼，仍由外掛自己決定。

### 關閉 {#shutting-down}

收到 `plugin.shutdown` 時，停止接受新連線，讓正在執行的請求在關閉時限內完成，然後關閉剩下的連線。WebSocket 這類長連線需要明確關閉。

## 使用 `rpc` 提供服務 {#serving-with-rpc}

每個請求都以一次 `http.handle` 呼叫到達：

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

| 欄位 | 意義 |
| --- | --- |
| `method` | HTTP 方法。 |
| `path` | `/api/plugins/<外掛 ID>/http` 之後的路徑。 |
| `query` | 不帶 `?` 的查詢字串。 |
| `headers` | 請求標頭，每個值都是字串陣列。 |
| `body_base64` | 請求本文，base64 編碼。 |
| `user` | 已登入的使用者：`id` 和 `name`。 |

回覆包含 `status`、`headers`（字串陣列）和 `body_base64`：

```json
{ "jsonrpc": "2.0", "id": 9, "result": { "status": 200, "headers": { "Content-Type": ["application/json"] }, "body_base64": "eyJvayI6dHJ1ZX0=" } }
```

整個請求和回覆都在一則訊息中傳輸，請保持較小的體積（單則訊息上限 4 MiB）。
