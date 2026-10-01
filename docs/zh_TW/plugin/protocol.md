---
outline: [2, 3]
---

# 通訊協定

Nginx UI 與外掛進程透過進程的標準輸入和標準輸出交換 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) 訊息。雙方都會傳送請求：Nginx UI 呼叫生命週期和能力方法，外掛呼叫宿主 API。外掛還可以透過 gRPC 提供能力呼叫，這在負載較大時更快，而存取日誌串流必須使用 gRPC。

::: tip 提示
SDK 實作了本頁的全部內容。不使用 SDK 撰寫外掛，或者想了解底層細節時，請閱讀本頁。
:::

## 訊息框架 {#framing}

- Nginx UI 寫入外掛的標準輸入，外掛寫入自己的標準輸出。
- 每則訊息是一行上的一個 JSON 物件，後接一個換行字元。訊息內部永遠不含原始換行字元。
- **標準輸出只能用於協定訊息。** 日誌、橫幅和堆疊追蹤都寫到標準錯誤，Nginx UI 會把它收集為外掛日誌，但從不解析它。
- 不支援批次訊息（JSON 陣列）。
- 一則訊息包括換行字元最多 **4 MiB**，更大的訊息會終止連線。

## 訊息 {#messages}

每則訊息都包含 `"jsonrpc": "2.0"`，其他成員決定它的類型：

| 類型 | `id` | `method` | `params` | `result` | `error` |
| --- | --- | --- | --- | --- | --- |
| 請求 | 存在且不為 `null` | 存在 | 選用 | | |
| 通知 | 不存在 | 存在 | 選用 | | |
| 成功回應 | 請求的 `id` | | | 存在 | |
| 錯誤回應 | 請求的 `id` | | | | 存在 |

- 每個請求恰好得到一個回應，通知永遠不會得到回應。
- `id` 是用於把回應和請求對應起來的不透明值，SDK 使用小整數。
- `params` 總是一個物件。

請求可以並行執行，並以任意順序得到回應，所以請用 `id` 對應回應。外掛可以並行處理請求，也可以逐一處理，但忙碌時仍必須回應健康檢查（參見[生命週期](./lifecycle.md#health-checks)）。

## 錯誤 {#errors}

```json
{ "jsonrpc": "2.0", "id": 7, "error": { "code": -32003, "message": "the API token is required", "data": { "field": "MYDNS_API_TOKEN" } } }
```

`message` 簡短易讀，絕不能包含認證。`data` 是選用的物件，包含詳細資料。

### 錯誤碼 {#error-codes}

| 代碼 | 名稱 | 使用情境 |
| --- | --- | --- |
| `-32700` | 解析錯誤 | 訊息不是有效的 JSON。 |
| `-32600` | 無效請求 | 訊息是 JSON，但不是有效的 JSON-RPC 訊息。 |
| `-32601` | 方法不存在 | 該方法沒有處理程式。 |
| `-32602` | 參數無效 | `params` 的結構不符合預期，或者在能力頁面說明的情況下參照了方法不認識的內容。 |
| `-32000` | 內部錯誤 | 呼叫已執行，但因其他原因失敗：服務商故障、網路故障。 |
| `-32001` | 權限拒絕 | 外掛權限未涵蓋的宿主 API 呼叫。 |
| `-32002` | 不支援 | 外掛沒有實作已宣告能力中的某個選用方法。 |
| `-32003` | 設定無效 | 使用者輸入的值缺少或錯誤。請把 `data.field` 設為該欄位的鍵。 |

::: tip 提示
`-32003` 與 `-32000` 的區別很重要：Nginx UI 會把 `-32003` 顯示在出錯的欄位旁邊，方便使用者修正；而 `-32000` 被視為服務故障。不要在 `-32000` 到 `-32099` 範圍內自行定義其他代碼。
:::

## JSON 結構 {#json-shapes}

每個 `params` 和 `result` 都是 plugin-spec 儲存庫中 [proto 檔案](https://github.com/nginxui/plugin-spec/tree/main/proto/nginxui/plugin/v1)所定義訊息的 [protobuf JSON 對應](https://protobuf.dev/programming-guides/json/)。本指南的各個頁面都以 JSON 展示每則訊息，所以只有在產生程式碼時才需要 proto 檔案。

- 成員名稱使用 `snake_case`，例如 `effective_fqdn`，從不使用 `camelCase`。
- 缺少的成員與取預設值（`""`、`0`、`false`、`[]`、`{}`、`null`）的成員意義相同。傳送方可以省略預設值，接收方把缺少的成員視為預設值。
- 數字是 JSON 數字。可能超過 4 GiB 的位元組數是雙精度浮點數，在 2^53 以內是精確的；`5242880` 和 `5242880.0` 都要接受。
- 二進位資料使用帶填補的 base64。
- 沒有參數的方法可以省略 `params`，沒有結果的方法回覆 `{}`。
- **忽略不認識的成員。** 協定正是透過新增選用成員來擴充而不破壞舊外掛的。

## 連線的生命週期 {#connection-lifetime}

連線就是進程的標準輸入和標準輸出。進程結束、任一方關閉自己那一端，或者訊息過大時，連線結束。新進程意味著新連線和新交握。

## gRPC 傳輸 {#grpc-transport}

外掛還可以透過 [gRPC](https://grpc.io) 提供能力呼叫。方法、訊息、錯誤和結果都不變，只是傳輸方式不同。Nginx UI 能用時就用 gRPC，否則退回標準輸入輸出，因此外掛在標準輸入輸出上也必須繼續提供每個方法。

### 啟用 {#opting-in}

外掛在交握回覆的 `transports` 中列出 `grpc`，並說明監聽位置：

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

- **Unix 套接字。** 外掛預設監聽 `<NGINX_UI_PLUGIN_DATA_DIR>/rpc.sock`。套接字路徑在 macOS 和 BSD 上限制為 104 位元組，在 Linux 上為 108 位元組；預設路徑放不下時，外掛在系統暫存目錄下建立一個私有目錄（權限 `0700`）並在其中監聽，然後在 `rpc_socket` 中回報路徑。監聽前刪除殘留的套接字檔案，以 `0600` 權限建立套接字，並在結束時刪除它。
- **具名管道。** 在 Windows 上，外掛以隨機名稱建立一個具名管道並在 `rpc_pipe` 中回報，同時回報 `rpc_token`（至少 128 位元的隨機值）。管道遵循與 [HTTP 管道](./http.md#where-to-listen)相同的規則。Nginx UI 在每次呼叫中傳送 `authorization: Bearer <rpc_token>`，外掛以 `UNAUTHENTICATED` 拒絕不帶這個值的呼叫。
- **回送 TCP。** 無法開啟具名管道的 Windows 外掛改為監聽 `127.0.0.1`，並回報 `rpc_port` 和 `rpc_token`，權杖檢查相同。

監聽必須在外掛回覆交握之前就能接受連線。隨後 Nginx UI 透過 gRPC 傳送 `plugin.ping` 檢查通道；外掛沒有列出 `grpc` 或檢查失敗時，該進程在整個生命週期內都使用標準輸入輸出。

### 各類流量的傳輸方式 {#what-travels-where}

| 流量 | 傳輸方式 |
| --- | --- |
| 交握、`plugin.initialized`、`plugin.shutdown`、`plugin.exit` | 一律使用標準輸入輸出 |
| `plugin.configure` 和健康檢查 | 標準輸入輸出 |
| 宿主 API 呼叫及其回覆 | 標準輸入輸出 |
| 事件和排程呼叫 | 標準輸入輸出 |
| 能力呼叫（`dns01.*`、`http.handle`、`notify.*`、`probe.check`、`mcp.call`、`storage.*`、`deploy.*`、`blocklist.fetch`、`discovery.resolve`） | 通道正常時使用 gRPC，否則使用標準輸入輸出 |
| 串流（`log.push`） | 只使用 gRPC |

提供 gRPC 的外掛必須在 gRPC 上提供每個能力方法和 `plugin.ping`，並且在兩種傳輸方式上回傳相同的結果或錯誤。如果交握和停止方法透過 gRPC 到達，外掛應忽略它們。

進程執行期間通道中斷時，Nginx UI 會在該進程剩餘的生命週期內改用標準輸入輸出。以 `UNAVAILABLE` 失敗且不帶外掛錯誤詳細資料的呼叫會在標準輸入輸出上重試一次，所以外掛自己不要回傳 `UNAVAILABLE`。

### gRPC 訊息與錯誤 {#grpc-messages-and-errors}

gRPC 服務路徑來自 proto 檔案，例如 `/nginxui.plugin.v1.DNS01/Present`，訊息使用標準的 protobuf 編碼，因此產生的 stub 程式碼可以直接使用。雙向都接受最大 64 MiB 的訊息。Nginx UI 會把自己的截止時間作為 gRPC 截止時間傳遞，截止時間到達後請停止處理該呼叫。

失敗的呼叫帶有 gRPC 狀態碼，並附帶一個 `nginxui.plugin.v1.PluginError` 狀態詳細資料，其中包含 JSON-RPC 的 `code` 和 `data`：

| JSON-RPC 代碼 | gRPC 狀態碼 |
| --- | --- |
| `-32601` 方法不存在、`-32002` 不支援 | `UNIMPLEMENTED` |
| `-32602` 參數無效、`-32003` 設定無效 | `INVALID_ARGUMENT` |
| `-32001` 權限拒絕 | `PERMISSION_DENIED` |
| `-32000` 內部錯誤 | `INTERNAL` |
| 其他代碼 | `UNKNOWN` |

接收方優先使用詳細資料中的代碼，沒有詳細資料時再根據狀態碼反向對應。

### 串流 {#streams}

有些流量是連續的資料流而不是一次呼叫，目前唯一的是 `log.push`，它把存取日誌行交給外掛（參見[存取日誌串流](./capabilities/log-sink.md)）。它是用戶端串流式 gRPC 呼叫：Nginx UI 傳送任意數量的訊息後關閉傳送端，外掛回覆一次。串流沒有 JSON-RPC 形式：它從不透過標準輸入輸出傳輸，在那裡它的方法名稱會像任何未知方法一樣得到 `-32601`。
