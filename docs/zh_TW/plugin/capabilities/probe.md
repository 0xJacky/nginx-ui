---
outline: [2, 3]
---

# 健康檢查

`probe` 外掛在內建的 HTTP 和 gRPC 檢查之外，加入檢查網站是否健康的方式：TCP 橫幅、資料庫登入、狀態 API。它的類型會作為網站健康檢查的選項出現，Nginx UI 依檢查的排程執行所選的類型。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `probe.check` | 是 | 對一個目標執行一次檢查。 |

`probe` 外掛接收呼叫不需要任何權限，但由於它要存取目標，應請求 `network`。

## 宣告類型 {#declaring-kinds}

```json [plugin.json]
"capabilities": ["probe"],
"permissions": ["network"],
"probe": {
  "kinds": [
    {
      "code": "tcp-banner",
      "name": "TCP banner",
      "configuration": {
        "fields": [
          { "key": "port", "type": "number", "display_name": "Port", "required": true },
          { "key": "expect", "display_name": "Expected banner prefix", "help_text": "For example SSH-2.0" }
        ]
      }
    }
  ]
}
```

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `code` | 是 | 類型的識別碼，參見[命名規則](../naming.md#provider-and-kind-codes)。在清單內唯一。 |
| `name` | 是 | 類型名稱。 |
| `configuration.fields` | 否 | 檢查表單的欄位，參見[設定表單](./notify.md#configuration-form)。 |

## 檢查 {#checking}

```json
{
  "jsonrpc": "2.0", "id": 34, "method": "probe.check",
  "params": { "kind": "tcp-banner", "target": "https://example.com", "config": { "port": "22", "expect": "SSH-2.0" }, "timeout_seconds": 10 }
}
```

| 欄位 | 意義 |
| --- | --- |
| `kind` | 類型的 `code`。 |
| `target` | 檢查的對象：網站網址，或健康檢查的自訂目標。類型只使用它需要的部分，例如 TCP 檢查只用主機名稱。 |
| `config` | 檢查表單的值。請當作機密處理。 |
| `timeout_seconds` | Nginx UI 等待的時長。請在此時間內完成。 |

```json
{ "jsonrpc": "2.0", "id": 34, "result": { "status": "down", "latency_ms": 10000, "message": "no banner within 10s" } }
```

| 欄位 | 意義 |
| --- | --- |
| `status` | `up`、`down` 或 `degraded`。 |
| `latency_ms` | 檢查耗時，不能為負數。 |
| `message` | 顯示給使用者的詳細資料，`status` 不是 `up` 時應當提供。絕不能包含認證。 |

`degraded` 表示目標有回應，但狀況不如預期：回應慢、部分失敗或者給出了警告。目標當機或沒有及時回應是一個**結果**：請回報 `down` 並附上訊息。只有檢查本身無法執行時才回覆錯誤：設定無法使用時回覆帶 `data.field` 的 `-32003`，其他情況回覆 `-32000`。

Nginx UI 會把呼叫失敗（錯誤、逾時、外掛無法使用）與 `down` 區分顯示，使使用者能分辨是目標當機還是檢查沒有執行。它把 `degraded` 視為上線並附帶訊息，並在 `timeout_seconds` 之外再多等待五秒。
