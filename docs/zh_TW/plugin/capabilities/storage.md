---
outline: [2, 3]
---

# 儲存

`storage` 外掛把檔案儲存到 Nginx UI 本身無法存取的地方：WebDAV 共用、SFTP 伺服器、雲端硬碟。它的後端會出現在內建儲存旁邊，Nginx UI 把它們用作自動備份的目的地。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `storage.put` | 是 | 以某個鍵儲存檔案。 |
| `storage.get` | 是 | 把某個鍵下的物件取回到檔案中。 |
| `storage.list` | 是 | 列出鍵以某個前綴開頭的物件。 |
| `storage.delete` | 是 | 刪除某個鍵下的物件。 |
| `storage.validate` | 否 | 在不儲存任何內容的情況下檢查設定。 |

檔案內容從不在訊息中傳輸。Nginx UI 與外掛以檔案的形式交換內容，因此幾 GB 的備份也不會在任何一方的記憶體中多佔一份。外掛應請求 `network`。

## 宣告後端 {#declaring-backends}

```json [plugin.json]
"capabilities": ["storage"],
"permissions": ["network"],
"storage": {
  "backends": [
    {
      "code": "webdav",
      "name": "WebDAV",
      "configuration": {
        "fields": [
          { "key": "url", "display_name": "Server URL", "help_text": "Folder the objects are kept in", "required": true },
          { "key": "username", "display_name": "Username", "required": true },
          { "key": "password", "display_name": "Password", "required": true, "secret": true }
        ]
      }
    }
  ]
}
```

`code`、`name` 和 `configuration.fields` 的用法與[通知通道](./notify.md#configuration-form)相同。

## 鍵 {#keys}

鍵命名一個物件。它是由 `/` 分隔的相對路徑，最長 1024 位元組，不能為空，不能以 `/` 開頭或結尾，不能包含空段、`.` 或 `..` 段、`\` 或控制字元。鍵區分大小寫，使用 UTF-8，可以包含任何文字。請視需要把鍵對應成服務自己的命名方式（例如百分比編碼），並在 `storage.list` 中原樣回傳。對其他形式的鍵回覆 `-32602`。

## 交換檔案 {#exchanging-files}

檔案透過外掛資料目錄中的 `exchange` 目錄傳遞。每次需要傳遞檔案的呼叫，Nginx UI 都會新建一個隨機命名的子目錄，並傳入其中的絕對路徑：

- 對於 `storage.put`，呼叫前檔案已位於 `source_path`。只能讀取它：不要修改、移動或刪除。
- 對於 `storage.get`，`target_path` 尚不存在。請把完整的物件建立為一個一般檔案，失敗時刪除不完整的檔案。

::: warning 注意
不要存取交換目錄中的任何其他路徑，回覆之後也不要保留或使用這些路徑：呼叫回傳後 Nginx UI 會刪除該子目錄。無法開啟或建立檔案的外掛回覆 `-32000`。
:::

## 儲存 {#storing}

```json
{
  "jsonrpc": "2.0", "id": 42, "method": "storage.put",
  "params": {
    "backend": "webdav",
    "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" },
    "key": "nginx-ui/daily_1790000000.zip",
    "source_path": "/var/lib/nginx-ui/plugins/.data/io.github.example.webdav/exchange/5f0c2a9d/daily_1790000000.zip"
  }
}
```

以 `key` 儲存完整的檔案，取代已有的物件，並且只在它持久儲存後才回覆：`{ "size": <已儲存的位元組數> }`。儲存失敗時不應留下不完整的物件。原因在於設定（密碼錯誤、資料夾不存在）時回覆帶 `data.field` 的 `-32003`，其他情況（服務故障、磁碟已滿、網路故障）回覆 `-32000`。

## 取回 {#fetching}

`storage.get` 包含 `backend`、`config`、`key` 和 `target_path`。把完整的物件寫入 `target_path`，並回覆 `{ "size": <已寫入的位元組數> }`。鍵下沒有物件時回覆 `-32000`，並在訊息中說明。

## 列出 {#listing}

::: code-group

```json [請求]
{ "jsonrpc": "2.0", "id": 44, "method": "storage.list", "params": { "backend": "webdav", "config": {}, "prefix": "nginx-ui/daily_" } }
```

```json [回應]
{ "jsonrpc": "2.0", "id": 44, "result": { "objects": [
  { "key": "nginx-ui/daily_1790000000.zip", "size": 5242880, "modified_at": "2026-09-21T03:00:05Z" }
] } }
```
:::

`prefix` 是一般的字串前綴而不是目錄：`nginx-ui/daily_` 符合 `nginx-ui/daily_1.zip`，空前綴列出所有物件。列出所有符合項目（自行處理服務的分頁），順序不限。`modified_at` 是 RFC 3339 時間，服務不提供時為空。能區分時，請排除不是由你儲存的物件。沒有符合時回傳空清單。

## 刪除 {#deleting}

`storage.delete` 包含 `backend`、`config` 和 `key`。刪除物件並回覆 `{}`。刪除不存在物件的鍵也要成功，這樣 Nginx UI 可以重試清理。

## 驗證 {#validating}

`storage.validate` 包含 `backend` 和 `config`。在不儲存、修改或刪除任何內容的情況下檢查設定，最好也不要存取服務。對第一個問題回覆帶 `data.field` 的 `-32003`，否則回覆 `{}`。要確認服務是否接受這份設定，Nginx UI 會呼叫 `storage.list`。

絕不要把 `config` 的值或檔案內容寫入日誌。

## 備份如何使用它 {#how-backups-use-it}

- 備份工作的儲存路徑就是鍵前綴。每次執行儲存 `<prefix>/<name>_<unix time>.zip`，加密備份還會儲存 `<prefix>/<name>_<unix time>.zip.key`。
- 只保留最近若干份備份的工作，在每次執行後用自己的前綴呼叫 `storage.list`，並對較舊的備份呼叫 `storage.delete`。
- 還原時對封存檔案及其金鑰檔案呼叫 `storage.get`。
- 備份表單的測試按鈕先呼叫 `storage.validate`，再呼叫 `storage.list`。

`storage.put` 和 `storage.get` 最多等待 10 分鐘，其他方法最多等待 30 秒，Nginx UI 不會自行重試任何操作。
