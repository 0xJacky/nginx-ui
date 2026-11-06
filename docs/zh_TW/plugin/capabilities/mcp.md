---
outline: [2, 3]
---

# MCP 工具

`mcp` 外掛向 Nginx UI 的 [MCP 伺服器](../../guide/mcp.md)加入工具，使連線到 Nginx UI 的 AI 助理可以在內建工具之外呼叫它們。Nginx UI 以外掛 ID 衍生的名稱發佈每個工具，像對待自己的工具一樣對每次呼叫進行授權，然後把呼叫轉發給外掛。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `mcp.call` | 是 | 執行一個工具。 |

## 宣告工具 {#declaring-tools}

```json [plugin.json]
"capabilities": ["mcp"],
"permissions": ["mcp", "network"],
"mcp": {
  "tools": [
    {
      "name": "purge_cache",
      "description": "Purge cached paths of a CDN zone.",
      "input_schema": {
        "type": "object",
        "properties": {
          "zone": { "type": "string", "description": "Zone name, e.g. example.com" },
          "paths": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["zone"]
      }
    }
  ]
}
```

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `name` | 是 | 外掛內的工具名稱，符合 `^[a-z0-9][a-z0-9_-]{0,47}$`，唯一。 |
| `description` | 是 | 工具的作用，供助理決定是否使用。 |
| `input_schema` | 否 | 參數的 JSON Schema，其 `type` 為 `object`。沒有時工具不接受參數。 |

必須請求 `mcp` 權限，它會告訴核准外掛的人：AI 助理將能夠執行外掛的程式碼。`io.github.example.cdn` 的 `purge_cache` 工具發佈為 `io_github_example_cdn__purge_cache`，參見[命名規則](../naming.md#mcp-tool-names)。

## 執行工具 {#running-a-tool}

```json
{
  "jsonrpc": "2.0", "id": 37, "method": "mcp.call",
  "params": { "tool": "purge_cache", "arguments": { "zone": "example.com", "paths": ["/index.html"] } }
}
```

`tool` 是清單中的名稱，不帶發佈時的前綴；`arguments` 是用戶端傳送的內容。**請先驗證參數再使用**：參數來自 AI 助理，可能不符合 Schema。

```json
{ "jsonrpc": "2.0", "id": 37, "result": { "content": [{ "type": "text", "text": "Purged 1 path in zone example.com." }] } }
```

| 欄位 | 意義 |
| --- | --- |
| `content` | 回傳給用戶端的內容區塊，依序排列。唯一的類型是 `text`，帶有 `text`。 |
| `is_error` | 工具執行了但失敗了，`content` 說明原因。 |

工具執行後失敗（服務拒絕、區域不存在）時，回覆一個 `is_error: true` 的結果，使助理讀到說明並能自行修正。只有呼叫無法處理時才回覆協定錯誤：清單未宣告的工具或參數不是物件時回覆 `-32602`，其他情況回覆 `-32000`。`content` 絕不能包含認證。

## Nginx UI 如何發佈工具 {#how-nginx-ui-publishes-tools}

- 只有在外掛已啟用並持有 `mcp` 權限時才發佈它的工具；外掛被停用、解除安裝或正在等待新權限核准時撤回。已連線的用戶端會收到工具清單變化的通知。
- 每個外掛工具都被視為會改變狀態的工具：呼叫需要存取權杖的 `mcp:write` 範圍，或已登入使用者的安全工作階段。
- 錯誤、逾時和外掛無法使用會以帶 `isError` 的工具結果回傳給用戶端。`mcp.call` 最多等待 60 秒。
