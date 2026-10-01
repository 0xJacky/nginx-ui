---
outline: [2, 3]
---

# 憑證部署

`cert.deploy` 外掛把 Nginx UI 簽發或續期的憑證推送到在 Nginx UI 之外終結 TLS 的地方：CDN、雲端負載平衡器、郵件伺服器、NAS。它宣告目標類型；使用者設定目標並把它們繫結到憑證，每當繫結的憑證簽發或續期，以及使用者手動觸發時，Nginx UI 都會呼叫外掛，並傳入憑證、私鑰和憑證鏈。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `deploy.push` | 是 | 把一張憑證推送到一個目標，或檢查能否推送。 |
| `deploy.validate` | 否 | 在不存取目標的情況下檢查目標設定。 |

外掛會收到私鑰，因此必須請求 `cert.deploy` 權限，它會如實告訴核准外掛的人這一點。外掛還應請求 `network`。

## 宣告目標類型 {#declaring-target-kinds}

```json [plugin.json]
"capabilities": ["cert.deploy"],
"permissions": ["cert.deploy", "network"],
"deploy": {
  "targets": [
    {
      "code": "mycdn",
      "name": "MyCDN",
      "configuration": {
        "fields": [
          { "key": "api_token", "display_name": "API token", "required": true, "secret": true },
          { "key": "zone_id", "display_name": "Zone ID", "help_text": "Zone the certificate is bound to", "required": true }
        ]
      }
    }
  ]
}
```

`code`、`name` 和 `configuration.fields` 的用法與[通知通道](./notify.md#configuration-form)相同。`code` 以 `kind` 的形式傳入。

## 推送 {#pushing}

```json
{
  "jsonrpc": "2.0", "id": 47, "method": "deploy.push",
  "params": {
    "kind": "mycdn",
    "config": { "api_token": "tok_live_xxx", "zone_id": "zone_123" },
    "certificate": {
      "name": "example.com",
      "domains": ["example.com", "www.example.com"],
      "certificate_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\n...\n-----END EC PRIVATE KEY-----\n",
      "chain_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "not_after": "2026-12-20T08:15:00Z"
    },
    "dry_run": false
  }
}
```

| 欄位 | 意義 |
| --- | --- |
| `kind`、`config` | 目標類型及其表單的值。 |
| `certificate.name` | 憑證在 Nginx UI 中的名稱。 |
| `certificate.domains` | 憑證涵蓋的網域名稱和 IP 位址。 |
| `certificate.certificate_pem` | 葉憑證。 |
| `certificate.private_key_pem` | 葉憑證的私鑰（PKCS #1、SEC 1 或 PKCS #8）。 |
| `certificate.chain_pem` | 中繼憑證，葉憑證的簽發者在前。沒有時為空。 |
| `certificate.not_after` | 葉憑證的到期時間，RFC 3339。 |
| `dry_run` | 只檢查，參見下文。 |

目標需要完整憑證鏈時，把 `certificate_pem` 和 `chain_pem` 串接起來。

目標接受憑證後回覆 `{ "message": "..." }`，附上一段變更摘要，例如建立或取代了哪個資源。推送必須是冪等的：推送目標已經在使用的憑證也要成功，因為 Nginx UI 會重試，使用者也可能再次推送。原因在於設定（權杖已撤銷、區域不存在）時回覆帶 `data.field` 的 `-32003`，其他情況回覆 `-32000`。

### 試執行 {#dry-run}

`dry_run: true` 時，不要修改目標上的任何內容。用唯讀呼叫檢查真正推送所需的條件（目標可存取、認證有效、資源存在），對必填欄位為空的設定回覆 `-32003`，並在 `message` 中說明真正推送時會做什麼。

### 保護私鑰 {#protecting-the-key}

::: danger 警告
- 私鑰和 `config` 中的每個值都是機密：絕不能出現在日誌、錯誤或 `message` 中。
- 不要把私鑰寫入磁碟，除非目標本身需要檔案（透過 SSH 推送的外掛把它寫到目標上，而不是自己的資料目錄中）。
- 回覆之後不要保留私鑰，並且只推送到本次呼叫的 `config` 指定的地方。
:::

## 驗證 {#validating}

`deploy.validate` 包含 `kind` 和 `config`。在不存取目標的情況下檢查設定：對第一個問題回覆帶 `data.field` 的 `-32003`，否則回覆 `{}`。Nginx UI 在儲存目標前呼叫它，並且不會儲存被它拒絕的目標。

## Nginx UI 如何使用它 {#how-nginx-ui-uses-it}

- 只有在外掛已啟用並持有 `cert.deploy` 權限時才提供它的目標類型。
- 目標可以繫結到一張憑證或所有憑證。繫結的憑證簽發或續期時，Nginx UI 把它推送到每個已啟用的目標，推送失敗時在 30 秒、2 分鐘和 10 分鐘後重試。每次嘗試都會重新讀取憑證。
- 使用者可以手動推送（嘗試一次，不重試），也可以用試執行測試目標。
- 每次推送都會連同訊息或錯誤以及嘗試次數一起記錄，並顯示在憑證旁邊。

`deploy.push` 最多等待 5 分鐘，`deploy.validate` 最多等待 30 秒。
