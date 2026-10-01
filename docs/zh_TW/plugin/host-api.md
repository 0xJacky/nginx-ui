---
outline: [2, 3]
---

# 宿主 API

交握完成後，外掛可以透過 `host.*` 方法呼叫 Nginx UI。它們是外掛發給 Nginx UI 的一般 JSON-RPC 請求，透過標準輸入輸出傳輸。Nginx UI 也會主動呼叫外掛，用於已訂閱的事件和排程工作。

| 方法 | 權限 | 用途 |
| --- | --- | --- |
| `host.log` | 無 | 寫入一筆結構化日誌。 |
| `host.kv.get`、`set`、`delete`、`list` | `kv` | 使用外掛的鍵值儲存。 |
| `host.settings.get` | 無 | 讀取外掛目前的設定。 |
| `host.i18n.locale` | 無 | 讀取介面語言。 |
| `host.credentials.get` | `credentials.read:<kind>` | 讀取已儲存的認證。 |
| `host.cron.register`、`unregister` | `cron` | 在執行期間安排排程呼叫。 |
| `host.notify` | `notify` | 在 Nginx UI 中發出通知。 |
| `host.metrics.snapshot` | `metrics.read` | 讀取 Nginx UI 的指標。 |
| `host.logs.list` | `log.files` | 列出外掛可以讀取的 nginx 日誌檔案。 |
| `host.activity.set` | 無 | 在處理指示器中顯示背景工作。 |
| `host.nginx.snippet.put`、`delete`、`list` | `nginx.snippet` | 維護 nginx 設定片段。 |
| `host.nginx.config.list`、`get` | `nginx.config.read` | 讀取 nginx 設定檔。 |
| `host.sites.list` | `sites.read` | 列出站點。 |
| `host.certs.list` | `certs.read` | 列出憑證。 |

::: info 說明
只有在 `plugin.initialized` 之後，並且只有外掛實際持有相應權限時，才能呼叫這些方法（參見[權限與安全](./permissions.md#granted-and-requested-permissions)）。沒有權限的呼叫會以 `-32001` 失敗，並且不執行任何操作。
:::

## 日誌 {#logging}

```json
{ "jsonrpc": "2.0", "id": 20, "method": "host.log", "params": { "level": "info", "message": "reconfigured", "fields": { "settings_count": 2 } } }
```

`level` 為 `debug`、`info`、`warn` 或 `error`，`fields` 是選用的結構化上下文。Nginx UI 回覆 `{}`，並把這一行與外掛的其他日誌輸出一起顯示。寫到標準錯誤的內容也會出現在那裡。絕不要記錄認證。

## 鍵值儲存 {#key-value-store}

每個外掛都有自己的鍵值儲存。鍵是非空字串，值可以是任意 JSON 值，編碼後最大 **64 KiB**。

::: code-group

```json [請求]
{ "jsonrpc": "2.0", "id": 21, "method": "host.kv.set", "params": { "key": "last_sync", "value": { "at": 1732000000 } } }
{ "jsonrpc": "2.0", "id": 22, "method": "host.kv.get", "params": { "key": "last_sync" } }
```

```json [回應]
{ "jsonrpc": "2.0", "id": 22, "result": { "value": { "at": 1732000000 }, "found": true } }
```
:::

| 方法 | 參數 | 結果 |
| --- | --- | --- |
| `host.kv.get` | `key` | `value` 和 `found`。鍵不存在時 `found: false`，不算錯誤。 |
| `host.kv.set` | `key`、`value` | `{}`。值過大時以 `-32602` 失敗。 |
| `host.kv.delete` | `key` | `{}`，無論鍵是否存在。 |
| `host.kv.list` | 選用的 `prefix` | `keys` 陣列，沒有符合時為空陣列。 |

兩個外掛永遠看不到彼此的鍵。較大或結構化的資料請改用外掛資料目錄中的檔案。

## 設定與語言 {#settings-and-language}

`host.settings.get` 回傳最近一次交握或 `plugin.configure` 中的設定，外掛不需要自己保存一份：

```json
{ "jsonrpc": "2.0", "id": 23, "result": { "settings": { "timeout": 120 } } }
```

`host.i18n.locale` 回傳介面語言，例如 `{ "locale": "zh_CN" }`，可用於在地化通知或日誌訊息。

## 認證 {#credentials}

`host.credentials.get` 依類型和 ID 讀取 Nginx UI 中儲存的認證：

::: code-group

```json [請求]
{ "jsonrpc": "2.0", "id": 25, "method": "host.credentials.get", "params": { "kind": "dns", "id": "7" } }
```

```json [回應]
{ "jsonrpc": "2.0", "id": 25, "result": { "id": "7", "name": "Cloudflare - example.com", "provider_code": "cloudflare", "config": { "CF_DNS_API_TOKEN": "..." } } }
```
:::

權限中寫明了類型，例如 `credentials.read:dns`，並且只涵蓋該類型。DNS-01 外掛很少需要這個方法：憑證使用的認證已經在每次呼叫中傳入。

## 排程工作 {#scheduled-tasks}

外掛可以讓 Nginx UI 依排程呼叫它的某個方法。固定的排程在清單中宣告，不需要權限：

```json [plugin.json]
"cron": [
  { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" }
]
```

也可以在持有 `cron` 權限時於執行期間註冊：

```json
{ "jsonrpc": "2.0", "id": 26, "method": "host.cron.register", "params": { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" } }
```

- `schedule` 是五段式 cron 運算式或 `@every <時長>`。
- `method` 是外掛提供的方法，最好使用外掛自己的名稱，例如 `catalog.refresh`。
- 再次註冊同一個 `id` 會取代先前的項目。`host.cron.unregister` 使用 `{ "id": ... }` 刪除項目，即使項目不存在也回覆 `{}`。

排程觸發時，Nginx UI 向該方法傳送一個一般請求：

```json
{ "jsonrpc": "2.0", "id": 31, "method": "catalog.refresh", "params": { "type": "refresh-catalog", "ts": 1732000000 } }
```

`type` 是項目 ID，`ts` 是觸發時的 Unix 時間。請回覆這個請求；結果會被忽略，錯誤會被記錄但不會重試。每次呼叫 Nginx UI 最多等待 **10 分鐘**，會為此啟動 `on_demand` 外掛，並且在上一次呼叫仍在執行時不會再次觸發同一項目。

## 通知 {#notifications}

```json
{ "jsonrpc": "2.0", "id": 27, "method": "host.notify", "params": { "level": "warning", "title": "Propagation slow", "content": "example.com is taking longer than usual to propagate.", "details": { "domain": "example.com" } } }
```

`level` 為 `info`、`success`、`warning` 或 `error`。通知會出現在 Nginx UI 的通知中，`details` 是視需要顯示的選用上下文。Nginx UI 把通知加入佇列後回覆 `{}`。

`notify` 權限與 `notify` 能力無關，後者讓外掛負責傳送通知，參見[通知通道](./capabilities/notify.md)。

## 指標 {#metrics}

`host.metrics.snapshot` 回傳 Nginx UI 目前的指標，例如 `{ "snapshot": { "cpu_percent": 3.2, "memory_bytes": 104857600 } }`。其結構不保證在各版本之間維持不變，請把它當作盡力而為的遙測資料。

## 日誌檔案 {#log-files}

持有 `log.files` 權限的外掛可以自己讀取 nginx 日誌檔案。`host.logs.list` 回傳 Nginx UI 允許讀取的檔案：

```json
{ "jsonrpc": "2.0", "id": 29, "result": { "logs": [
  { "path": "/var/log/nginx/access.log", "type": "access", "source": "default" },
  { "path": "/var/log/nginx/example.com.error.log", "type": "error", "source": "config", "config_file": "/etc/nginx/sites-enabled/example.com.conf" }
] } }
```

| 欄位 | 意義 |
| --- | --- |
| `path` | 目前日誌檔案的絕對路徑。 |
| `type` | `access` 或 `error`。 |
| `source` | nginx 設定中指定了該路徑時為 `config`，nginx 內建預設日誌為 `default`。 |
| `config_file` | 指定該路徑的設定檔。 |

輪替後的檔案（`access.log.1`、`access.log.2.gz`、`access.log-20260101`）不會列出，請在已列出路徑的旁邊尋找它們。只讀取已列出的檔案及其輪替副本，把內容視為個人資料，絕不寫入自己的日誌。nginx 設定變化時清單也會變化：請訂閱 `log.paths_changed`，收到後再次呼叫 `host.logs.list`。

如果想在 nginx 寫入時接收新日誌行而不是讀取檔案，參見[存取日誌串流](./capabilities/log-sink.md)。

## nginx 設定 {#nginx-configuration}

### 設定片段 {#snippets}

持有 `nginx.snippet` 權限的外掛可以維護屬於自己的 nginx 設定，例如快取規則或流量限制。`host.nginx.snippet.put` 寫入一個片段：

::: code-group

```json [請求]
{ "jsonrpc": "2.0", "id": 31, "method": "host.nginx.snippet.put", "params": { "name": "static-cache", "content": "location ~* \\.(css|js)$ {\n  expires 7d;\n}\n" } }
```

```json [回應]
{ "jsonrpc": "2.0", "id": 31, "result": { "changed": true, "include": "include snippets/plugins/io.github.example.cache/static-cache.conf;" } }
```

:::

- `name` 為 1 到 64 個 `[a-z0-9_-]` 字元，以字母或數字開頭；`content` 最多 256 KiB 的 UTF-8 文字。每個外掛最多保留 32 個片段。
- Nginx UI 寫入片段，測試整體設定並重新載入 nginx。nginx 拒絕設定時，先前的片段保持不變，不會重新載入，呼叫以 `-32602` 失敗並附上 nginx 的輸出。
- 片段內容沒有變化時 `changed` 為 `false`，此時也不會重新載入。

片段只在被引入的位置生效。請把 `include` 展示給使用者（例如在你的設定面板中），由使用者把它加到合適的 `server` 或 `location` 區塊中。Nginx UI 永遠不會自行加入它。

`host.nginx.snippet.list` 回傳 `{ "snippets": [{ "name": "...", "include": "..." }] }`；`host.nginx.snippet.delete` 傳入 `{ "name": "..." }` 刪除一個片段，並回覆 `{ "removed": true }`。仍被引入的片段無法刪除：否則 nginx 會拒絕設定，因此片段保留，呼叫以 `-32602` 失敗。

::: info 說明
停用外掛時片段會保留，nginx 繼續提供原有服務。解除安裝外掛時，Nginx UI 會刪除它的片段；仍被引入的片段會被清空，以保持設定有效。
:::

### 讀取設定 {#reading-the-configuration}

持有 `nginx.config.read` 時，`host.nginx.config.list` 回傳相對於 nginx 設定目錄的設定檔，例如 `["conf.d/gzip.conf", "nginx.conf", "sites-available/example.com"]`；`host.nginx.config.get` 傳入 `{ "path": "nginx.conf" }` 回傳 `{ "content": "..." }`。清單包含 `nginx.conf`、所有 `.conf` 檔案以及 `sites-available` 和 `streams-available` 中的檔案。金鑰、密碼檔和符號連結不在其中，單一檔案最大 1 MiB。

## 站點與憑證 {#sites-and-certificates}

`host.sites.list`（`sites.read`）回傳站點：

```json
{ "sites": [ { "name": "example.com", "status": "enabled", "urls": ["https://example.com"], "config_file": "sites-available/example.com" } ] }
```

`status` 為 `enabled`、`disabled` 或 `maintenance`，`config_file` 可以用 `host.nginx.config.get` 讀取。

`host.certs.list`（`certs.read`）回傳憑證，從不包含私鑰：

```json
{ "certs": [ { "id": "3", "name": "example.com", "domains": ["example.com", "www.example.com"], "auto_renew": true, "challenge_method": "dns01", "key_type": "P256", "not_before": "2026-09-01T00:00:00Z", "not_after": "2026-11-30T00:00:00Z", "issuer": "Let's Encrypt" } ] }
```

憑證檔案無法讀取時，日期和簽發者為空。訂閱 `site.*` 和 `cert.*` [事件](#events)，以便知道何時重新取得清單。

## 處理指示器 {#processing-indicator}

`host.activity.set` 在 Nginx UI 的處理指示器中顯示外掛的背景工作，與 Nginx UI 自己的工作並列：

```json
{ "jsonrpc": "2.0", "id": 30, "method": "host.activity.set", "params": { "key": "indexing", "label": "Indexing access logs", "active": true } }
```

- `key` 在外掛內識別項目：1 到 64 個 `[a-z0-9._-]` 字元。
- `label` 是 1 到 128 個字元的英文文字。Nginx UI 會用外掛瀏覽器套件註冊的翻譯來翻譯它，鍵就是這段英文。
- `active: true` 顯示或更新項目，`active: false` 刪除項目。

Nginx UI 為每個外掛至少允許 8 個項目，並在外掛停止、當機或被停用時刪除它的全部項目。

## 事件 {#events}

外掛在清單的 `events` 陣列中訂閱 Nginx UI 的事件：

```json [plugin.json]
"events": ["cert.renewed", "nginx.reload_failed"]
```

Nginx UI 以 `events.on` 通知傳送每個事件，通知永遠不需要回覆：

```json
{ "jsonrpc": "2.0", "method": "events.on", "params": { "type": "cert.renewed", "data": { "domain": "example.com" }, "ts": 1732000000 } }
```

| 事件 | 觸發時機 |
| --- | --- |
| `cert.issued` | 憑證簽發完成。 |
| `cert.renewed` | 憑證已續期。 |
| `cert.expiring` | 憑證即將到期。 |
| `site.saved` | 網站設定已儲存。 |
| `site.enabled` | 網站已啟用。 |
| `site.disabled` | 網站已停用。 |
| `nginx.reloaded` | nginx 已重新載入。 |
| `nginx.reload_failed` | nginx 重新載入失敗。 |
| `node.status_changed` | 叢集節點的狀態發生變化。 |
| `node.joined` | 節點加入叢集。 |
| `backup.completed` | 備份完成。 |
| `auth.login_failed` | 登入失敗。 |
| `plugin.changed` | 外掛被安裝、更新、啟用或停用。 |
| `log.paths_changed` | `host.logs.list` 回傳的檔案發生變化。只傳送給持有 `log.files` 的外掛。 |

只有已訂閱的事件才會傳送。請忽略不認識的事件類型：將來可能會新增事件。
