---
outline: [2, 3]
---

# 存取日誌串流

`log.sink` 外掛在 nginx 寫入存取日誌時即時接收 Nginx UI 的存取日誌行（已解析為欄位），並把它們傳送到任何地方：日誌儲存、SIEM、指標管線。Nginx UI 讀取日誌，把日誌行分批，並以串流的方式把每一批傳給外掛。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `log.push` | 是 | 接收一批項目並回覆一次。僅限 gRPC。 |

存取日誌產生的行數遠多於每行一個請求所能承載的量，因此 `log.push` 是 gRPC 串流，`log.sink` 外掛必須提供 [gRPC 傳輸](../protocol.md#grpc-transport)。存取日誌包含用戶端位址和每個請求的 URL，因此外掛必須請求 `log.read` 權限。

## 宣告能力 {#declaring-the-capability}

```json [plugin.json]
"capabilities": ["log.sink"],
"permissions": ["log.read", "network"],
"log_sink": {
  "batch_size": 512,
  "flush_interval_ms": 1000,
  "formats": ["combined"]
}
```

`log_sink` 區塊是選用的：

| 欄位 | 預設值 | 意義 |
| --- | --- | --- |
| `batch_size` | 256 | 一個串流中最多的項目數，0 到 4096。 |
| `flush_interval_ms` | 500 | 串流在第一個項目之後保持開啟的最長時間。`0` 或至少 50。 |
| `formats` | 所有行 | 外掛需要的日誌行格式。 |

串流中的項目達到 `batch_size`，或者自第一個項目起已過 `flush_interval_ms`，串流就會立即關閉。

| 格式 | 日誌行 |
| --- | --- |
| `combined` | nginx `combined` 格式的行，後面可以接 `$request_time` 和 `$upstream_response_time`。解析出的欄位都會設定。 |
| `raw` | 其他所有行：自訂 `log_format`、JSON 日誌、被截斷的行。只設定 `raw` 和 `timestamp`（讀取該行的時間）。 |

列出了格式的外掛永遠不會收到其他格式的行，包括將來新增的格式。

## 接收串流 {#receiving-a-stream}

外掛在交握中列出 `grpc`，並提供 `/nginxui.plugin.v1.LogSink/Push`。Nginx UI 開啟一個串流，每行傳送一則訊息，然後關閉傳送端；外掛讀到結尾後回覆一次。一則訊息的 JSON 形式如下：

```json
{
  "log_path": "/var/log/nginx/access.log",
  "entry": {
    "timestamp": "2026-09-23T08:15:02Z",
    "remote_addr": "203.0.113.7",
    "request_method": "GET",
    "request_uri": "/index.html?lang=en",
    "protocol": "HTTP/1.1",
    "status": 200,
    "body_bytes_sent": 612,
    "referer": "https://example.com/",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64)",
    "request_time": 0.004,
    "upstream_response_time": 0.003,
    "raw": "203.0.113.7 - - [23/Sep/2026:08:15:02 +0000] \"GET /index.html?lang=en HTTP/1.1\" 200 612 ...",
    "format": "combined"
  }
}
```

回覆統計外掛保留和捨棄的項目數：

```json
{ "accepted": 3, "rejected": 0 }
```

### 項目欄位 {#entry-fields}

| 欄位 | nginx 變數 | 意義 |
| --- | --- | --- |
| `timestamp` | `$time_local` | 請求時間，RFC 3339，UTC。 |
| `remote_addr` | `$remote_addr` | 用戶端位址。 |
| `request_method`、`request_uri`、`protocol` | 來自 `$request` | 方法、帶查詢字串的目標、協定。 |
| `status` | `$status` | 回應狀態。 |
| `body_bytes_sent` | `$body_bytes_sent` | 回應本文大小，單位位元組。 |
| `referer`、`user_agent` | `$http_referer`、`$http_user_agent` | 請求標頭。 |
| `upstream_addr` | `$upstream_addr` | 處理請求的上游伺服器。 |
| `request_time`、`upstream_response_time` | 同名變數 | 時間，單位秒。 |
| `host` | `$host` | 請求所屬的主機。 |
| `raw` | | nginx 寫入的原始行，一律會設定。 |
| `format` | | `combined` 或 `raw`。 |

Nginx UI 無法擷取的欄位不會出現。`combined` 格式既不包含 `$upstream_addr` 也不包含 `$host`，需要它們的外掛請自行解析 `raw`。

### 及時回覆 {#answering-promptly}

在收到回覆之前，Nginx UI 不會開啟下一個串流，等待期間還會捨棄日誌行，所以請盡快回覆：先暫存項目，在目的地確認之前就回覆。錯誤狀態表示外掛遺失了整批項目，這批項目不會重新傳送。

::: warning 注意
每個欄位都是不可信的輸入，因為目標、參照頁面和使用者代理都由用戶端決定；在許多司法管轄區，這些項目還屬於個人資料。絕不要把它們寫入自己的日誌。
:::

## Nginx UI 如何串流 {#how-nginx-ui-streams}

- 它串流外掛啟用期間寫入的日誌行，只來自 Nginx UI 日誌檢視器被允許讀取的日誌。它從不重播較早的行，也不會在日誌輪替時重複傳送同一行。
- 慢的外掛永遠不會拖慢 nginx 或其他外掛：每個外掛有一個 8192 筆的佇列，放不下的行會被捨棄並計數。
- 失敗的串流中的項目計為捨棄，下一個串流等待 1 秒，每次失敗後加倍，最長 30 秒。每個串流最長 30 秒。
- 外掛詳細資料顯示它接受和拒絕的項目數，以及 Nginx UI 捨棄的項目數。不提供 gRPC 的 `log.sink` 外掛收不到任何內容，並會回報原因。
