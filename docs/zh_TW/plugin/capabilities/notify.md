---
outline: [2, 3]
---

# 通知通道

`notify` 外掛透過自己的服務傳送 Nginx UI 的通知：聊天服務、推播閘道、告警系統。它的通道會出現在內建通知通道旁邊。使用者只需設定一次通道，之後每當有通知路由到該通道，Nginx UI 就會呼叫外掛。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `notify.send` | 是 | 傳送一則通知。 |
| `notify.validate` | 否 | 在不傳送任何內容的情況下檢查通道設定。 |

`notify` 能力不是 `notify` 權限：後者允許外掛用 `host.notify` 在 Nginx UI 中發出通知。`notify` 外掛接收 `notify.send` 不需要任何權限，但由於它要與外部服務通訊，應請求 `network`。

## 宣告通道 {#declaring-channels}

```json [plugin.json]
"capabilities": ["notify"],
"permissions": ["network"],
"notify": {
  "channels": [
    {
      "code": "mychat",
      "name": "MyChat",
      "configuration": {
        "fields": [
          { "key": "webhook_url", "display_name": "Webhook URL", "required": true },
          { "key": "token", "display_name": "Token", "help_text": "Bot token of the workspace", "required": true, "secret": true },
          { "key": "mention_all", "type": "bool", "display_name": "Mention everyone on errors" }
        ]
      }
    }
  ]
}
```

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `code` | 是 | 通道的識別碼，參見[命名規則](../naming.md#provider-and-kind-codes)。在清單內唯一。 |
| `name` | 是 | 通道名稱。 |
| `configuration.fields` | 否 | 通道表單的欄位。 |

## 設定表單 {#configuration-form}

通道、健康檢查類型、儲存後端、部署目標、封鎖清單來源和探索提供者都用同樣的 `configuration.fields` 描述表單：

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `key` | 是 | 值在 `config` 中的鍵。不能為空，在表單內唯一。 |
| `type` | 否 | `text`（預設）、`textarea`、`number` 或 `bool`。 |
| `display_name` | 是 | 欄位標籤。 |
| `help_text` | 否 | 欄位下方的說明。 |
| `required` | 否 | 該欄位為空時 Nginx UI 不會儲存設定。 |
| `secret` | 否 | 認證：遮罩顯示，從不記錄。 |

每個值都以字串的形式出現在 `config` 中：數字為十進位文字，例如 `"30"`，`bool` 為 `"true"` 或 `"false"`。空欄位可能不存在。Nginx UI 會加密儲存設定。

::: warning 注意
請把 `config` 中的**每個**值都當作機密，而不只是 `secret` 欄位：Webhook 網址中常常包含權杖。
:::

## 傳送 {#sending}

```json
{
  "jsonrpc": "2.0", "id": 30, "method": "notify.send",
  "params": {
    "channel": "mychat",
    "config": { "webhook_url": "https://chat.example/hooks/xxx", "token": "tok_live_xxx" },
    "title": "Certificate Expiring Soon",
    "content": "Certificate example.com expires in 7 days.",
    "severity": "warning"
  }
}
```

| 欄位 | 意義 |
| --- | --- |
| `channel` | 通道的 `code`。 |
| `config` | 通道表單的值。 |
| `title`、`content` | 通知內容，已翻譯成通道的語言。純文字：請依服務自己的格式進行跳脫。 |
| `severity` | `info`、`success`、`warning` 或 `error`。未知的值以 `info` 處理。 |

服務接受訊息後回覆 `{}`。失敗時，原因在於設定（權杖已撤銷、網址錯誤）則回覆 `-32003` 和 `data.field`，否則回覆 `-32000`。Nginx UI 不會重試：這則通知仍然顯示在 Nginx UI 中，也仍會透過其他通道傳送。`notify.send` 最多等待 30 秒。

## 驗證 {#validating}

```json
{ "jsonrpc": "2.0", "id": 33, "method": "notify.validate", "params": { "channel": "mychat", "config": { "webhook_url": "not a url" } } }
```

在不傳送任何內容、不存取服務的情況下檢查設定。對第一個缺少或格式錯誤的欄位回覆帶 `data.field` 的 `-32003`，否則回覆 `{}`。建立或修改通道時 Nginx UI 會呼叫它，並且不會儲存被它拒絕的設定。沒有實作這個方法的外掛回覆 `-32002`，視為沒有異議。
