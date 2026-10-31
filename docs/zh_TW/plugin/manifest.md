---
outline: [2, 3]
---

# 清單檔案

每個外掛的外掛套件根目錄都有一個 `plugin.json`。它是一個 UTF-8 編碼的 JSON 物件，說明外掛是什麼、包含什麼、請求什麼。Nginx UI 在安裝外掛前會驗證它，違反本頁任何規則的外掛套件都會被拒絕。

清單的 [JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/plugin.schema.json) 可以為編輯器提供自動完成和驗證：

```json
{
  "$schema": "https://raw.githubusercontent.com/nginxui/plugin-spec/main/schema/plugin.schema.json",
  "id": "io.github.example.mydns"
}
```

## 欄位 {#fields}

| 欄位 | 型別 | 必填 | 意義 |
| --- | --- | --- | --- |
| `id` | string | 是 | 唯一的外掛 ID。參見[識別](#identity)。 |
| `name` | string | 是 | 顯示名稱。 |
| `version` | string | 是 | 外掛本身的版本，使用[語意化版本](https://semver.org/lang/zh-TW/)。 |
| `description` | string | 否 | 外掛清單中顯示的一行簡介。 |
| `i18n` | object | 否 | `name`、`description`、`permission_reasons` 和螢幕截圖說明的翻譯。參見[翻譯](#translations)。 |
| `homepage_url` | string | 否 | 文件或儲存庫連結。 |
| `icon_path` | string | 否 | 外掛套件內圖示檔案的路徑。 |
| `api_version` | integer | 是 | 外掛使用的協定世代，目前固定為 `1`。 |
| `min_nginx_ui_version` | string | 否 | 外掛支援的最低 Nginx UI 版本。參見[版本與相容性](./versioning.md)。 |
| `server` | object | 否\* | 伺服器端進程。參見[伺服器端進程](#server-process)。 |
| `webapp` | object | 否\* | 瀏覽器套件或靜態頁面。參見[瀏覽器套件](./webapp.md)。 |
| `content` | object | 否\* | 範本和翻譯檔案。參見[範本與翻譯](./capabilities/content.md)。 |
| `capabilities` | string[] | 否 | 外掛實作的能力。參見[能力](#capabilities)。 |
| `permissions` | string[] | 否 | 外掛需要的 Nginx UI 功能。參見[權限與安全](./permissions.md)。 |
| `requires` | object[] | 否 | 外掛相依的其他外掛。參見[相依與衝突](#dependencies-and-conflicts)。 |
| `requires_capabilities` | string[] | 否 | 需要其他已啟用外掛提供的能力。 |
| `conflicts` | string[] | 否 | 絕不能與本外掛同時執行的外掛。 |
| `events` | string[] | 否 | 外掛訂閱的事件。參見[宿主 API](./host-api.md#events)。 |
| `cron` | object[] | 否 | 排程呼叫。參見[宿主 API](./host-api.md#scheduled-tasks)。 |
| `network_hosts` | string[] | 否 | 外掛打算連線的主機，會顯示給安裝外掛的人。 |
| `permission_reasons` | object | 否 | 外掛要求每項權限的原因。參見[說明權限用途](./permissions.md#explaining-permissions)。 |
| `screenshots` | object[] | 否 | 外掛目錄展示用的使用畫面截圖。參見[螢幕截圖](#screenshots)。 |
| `settings_schema` | object | 否 | 設定表單。參見[設定](#settings)。 |
| `dns01`、`http`、`notify`、`probe`、`mcp`、`storage`、`deploy`、`blocklist`、`discovery`、`log_sink` | object | 宣告對應能力時 | 各能力的設定。 |

\* 清單至少宣告 `server`、`webapp` 和 `content` 中的一個，否則外掛什麼也不會安裝。

::: info 說明
清單中的所有路徑（`icon_path`、`server.executables`、`webapp` 中的路徑、`content` 中的路徑）都相對於外掛套件根目錄，並且必須是[安全的相對路徑](./packaging.md#safe-paths)。螢幕截圖路徑是唯一的例外，它相對於儲存庫，參見[螢幕截圖](#screenshots)。
:::

## 識別 {#identity}

- `id` 由小寫字母、數字和連字號組成，至少兩段，以點分隔，最長 64 個字元：`^[a-z0-9]+(\.[a-z0-9-]+)+$`，例如 `io.github.example.mydns`。外掛的 ID 在整個生命週期內都不會改變。[命名規則](./naming.md#plugin-ids)說明了如何選擇 ID。
- `name` 必填且不能為空。
- `version` 遵循語意化版本 2.0.0，例如 `1.4.0` 或 `2.0.0-beta.1`。預發佈版本會讓該版本進入較不穩定的[發佈通道](./catalog.md#release-channels)。
- `api_version` 為 `1`。Nginx UI 會拒絕其他任何值。

## 翻譯 {#translations}

`i18n` 把名稱和描述翻譯成 Nginx UI 介面的語言：

```json [plugin.json]
"i18n": {
  "zh_CN": { "name": "DNS-01 验证", "description": "使用任意 DNS 服务商完成 ACME DNS-01 验证。" },
  "ja_JP": { "name": "DNS-01 チャレンジ" }
}
```

只需翻譯你想支援的語言。最上層的 `name` 和 `description` 是英文文字，沒有翻譯的語言會顯示它們。[`permission_reasons`](./permissions.md#explaining-permissions) 也可以用同樣的方式依語言翻譯。

鍵使用 Nginx UI 介面的語言代碼，例如 `zh_CN`、`zh_TW` 或 `ja_JP`。Nginx UI 顯示它介面已有的語言，其他語言顯示英文，所以外掛可以先於部分 Nginx UI 提供某種語言。[`nginx-ui plugin lint`](./rules.md#manifest-i18n) 會回報不是語言代碼的鍵，並對執行它的 Nginx UI 沒有的語言發出警告。

## 螢幕截圖 {#screenshots}

`screenshots` 最多列出 8 張外掛使用中的螢幕截圖，依序顯示在外掛目錄的展示頁上：

```json [plugin.json]
"screenshots": [
  {
    "id": "dashboard",
    "path": "docs/screenshots/dashboard.png",
    "dark_path": "docs/screenshots/dashboard-dark.png",
    "caption": "Traffic at a glance"
  }
],
"i18n": {
  "zh_TW": { "screenshot_captions": { "dashboard": "一覽流量" } }
}
```

和清單中的其他路徑不同，`path` 和 `dark_path` 相對於外掛儲存庫的根目錄，而不是外掛套件，這樣圖片不會打包進外掛套件。外掛目錄在每個版本的 tag 上讀取它們，所以發佈新版本就能更新螢幕截圖。請使用 PNG、JPEG 或 WebP 圖片，寬高比約 16:10，寬度 1280 到 1920 像素。`dark_path` 是同一畫面的深色主題版本，Nginx UI 介面為深色時顯示它。`id` 是螢幕截圖的名稱，由小寫字母、數字和連字號組成。`caption` 使用英文，`i18n` 中的 `screenshot_captions` 依 `id` 翻譯它。[`nginx-ui plugin lint`](./rules.md#manifest-screenshots) 會檢查路徑和說明。

## 伺服器端進程 {#server-process}

```json [plugin.json]
"server": {
  "executables": {
    "linux-amd64": "dist/linux-amd64/mydns",
    "linux-arm64": "dist/linux-arm64/mydns",
    "windows-amd64": "dist/windows-amd64/mydns.exe"
  },
  "lifecycle": "on_demand",
  "idle_timeout_seconds": 300,
  "resources": { "memory_mb": 256, "cpu_percent": 50, "recommended_memory_mb": 1024 }
}
```

| 欄位 | 意義 |
| --- | --- |
| `executables` | 平台鍵（`<goos>-<goarch>`，參見[命名規則](./naming.md#platform-keys)）到該平台可執行檔的對應。 |
| `command` | 直譯式外掛的命令列，例如 `["python3", "server/main.py"]`。當 `executables` 沒有目前平台的項目時使用。 |
| `lifecycle` | `resident`（預設）表示外掛啟用期間進程持續執行；`on_demand` 表示需要時才啟動。 |
| `idle_timeout_seconds` | 用於 `on_demand`：進程閒置多久後被 Nginx UI 停止。不能為負數。 |
| `resources` | 進程需要的資源。參見[資源](#resources)。 |

`server` 需要 `executables` 或 `command`。`command` 的第一個元素要麼是在 `PATH` 中尋找的程式（例如 `python3`），要麼是外掛套件內的路徑。Nginx UI 安裝外掛套件時會為每個宣告的可執行檔加上執行權限；如果目前平台既沒有可執行檔也沒有命令，外掛就無法啟動。

[生命週期](./lifecycle.md)一頁介紹了進程如何啟動、檢查和停止。

### 資源 {#resources}

`server.resources` 告訴 Nginx UI 和安裝外掛的人進程需要多少資源。三個值都是選用的，且不能為負數。

| 欄位 | 意義 |
| --- | --- |
| `memory_mb` | 進程最多使用的記憶體，單位 MiB。 |
| `cpu_percent` | 進程最多使用的 CPU 時間，以單一核心的百分比表示：`100` 為一個核心，`250` 為兩個半核心。 |
| `recommended_memory_mb` | 機器（或執行 Nginx UI 的容器）應有的記憶體，包括 Nginx UI 和外掛。 |

當 Nginx UI 限制外掛進程時，會取這些提示值和它本身限制中較小的一個，因此提示值只能降低限制，不能提高限制。請保留餘裕：超出記憶體限制的進程會被終止。`recommended_memory_mb` 只是建議，Nginx UI 會在選擇外掛的地方顯示它，並在機器記憶體不足時發出警告，但絕不會因此拒絕安裝外掛。參見[生命週期](./lifecycle.md#resource-limits)。

## 能力 {#capabilities}

`capabilities` 列出外掛實作的能力。每個名稱只能出現一次，且必須是以下之一：

| 名稱 | 需要的設定區塊 | 需要的權限 | 頁面 |
| --- | --- | --- | --- |
| `dns01` | 至少包含一個服務商的 `dns01` | | [DNS-01](./capabilities/dns01.md) |
| `http` | 包含 `listen` 的 `http` | | [HTTP 介面](./http.md) |
| `notify` | 至少包含一個通道的 `notify` | | [通知通道](./capabilities/notify.md) |
| `probe` | 至少包含一種類型的 `probe` | | [健康檢查](./capabilities/probe.md) |
| `mcp` | 至少包含一個工具的 `mcp` | `mcp` | [MCP 工具](./capabilities/mcp.md) |
| `storage` | 至少包含一個後端的 `storage` | | [儲存](./capabilities/storage.md) |
| `cert.deploy` | 至少包含一個目標的 `deploy` | `cert.deploy` | [憑證部署](./capabilities/cert-deploy.md) |
| `security.blocklist` | 至少包含一個來源的 `blocklist` | `network` | [封鎖清單](./capabilities/blocklist.md) |
| `upstream.discovery` | 至少包含一個提供者的 `discovery` | `network` | [上游探索](./capabilities/discovery.md) |
| `log.sink` | `log_sink`，選用 | `log.read` | [存取日誌串流](./capabilities/log-sink.md) |

所有能力都由進程提供，因此沒有 `server` 的清單不能宣告能力、`cron` 或 `events`。

一個外掛可以實作多個能力，也可以在之後的版本中追加：列出新的名稱，加上對應的設定區塊並實作它的方法。交握必須回報相同的集合，參見[生命週期](./lifecycle.md#handshake)。新能力需要外掛原本沒有的權限時，更新會等待核准，參見[權限](./permissions.md#granted-and-requested-permissions)。外掛不能自訂能力；超出上表範圍的功能請使用 [HTTP](./http.md)、[瀏覽器套件](./webapp.md)和[事件](./host-api.md#events)。

## 相依與衝突 {#dependencies-and-conflicts}

```json [plugin.json]
"requires": [{ "id": "com.nginxui.dns01", "version": ">=1.2.0" }],
"requires_capabilities": ["storage"],
"conflicts": ["io.github.other.mydns"]
```

- `requires` 列出必須先安裝並啟用的外掛。`id` 是外掛 ID，`version` 是選用的語意化版本範圍。相依外掛缺少時 Nginx UI 會拒絕啟用外掛。
- `requires_capabilities` 列出需要由其他已啟用外掛提供的能力。沒有外掛提供時 Nginx UI 會發出警告。
- `conflicts` 列出絕不能與本外掛同時執行的外掛，例如同一功能的另一種實作。每一項都必須是有效的外掛 ID，不能是本外掛自己的 ID，不能重複，也不能同時出現在 `requires` 中。衝突關係是雙向的：任一方列出另一方，兩者就衝突。衝突的外掛已啟用時，Nginx UI 會拒絕啟用本外掛，除非使用者選擇取代它。

## 設定 {#settings}

`settings_schema` 描述由 Nginx UI 為外掛產生的設定表單。設定值會在交握和每次 `plugin.configure` 呼叫中傳給進程，參見[生命週期](./lifecycle.md#settings)。

```json [plugin.json]
"settings_schema": {
  "header": "Defaults for every certificate.",
  "settings": [
    { "key": "timeout", "type": "number", "display_name": "Timeout", "help_text": "Seconds to wait for the API.", "default": 30 },
    { "key": "region", "type": "select", "display_name": "Region", "options": [
      { "value": "eu", "label": "Europe" },
      { "value": "us", "label": "United States" }
    ] },
    { "key": "api_key", "type": "secret", "display_name": "API key", "required": true }
  ]
}
```

| 欄位 | 意義 |
| --- | --- |
| `header`、`footer` | 表單上方和下方的文字。 |
| `settings[].key` | 值的鍵，在清單內唯一。 |
| `settings[].type` | `text`、`bool`、`number`、`select`、`secret`、`textarea` 或 `list`。 |
| `settings[].display_name` | 欄位標籤。 |
| `settings[].help_text` | 欄位下方的說明。 |
| `settings[].default` | 預設值。`list` 型別為字串陣列。 |
| `settings[].options` | `{ "value", "label" }` 選項，`select` 型別必填。 |
| `settings[].required` | 該欄位必須填寫。 |

`list` 欄位儲存字串陣列，顯示為可編輯的清單。`secret` 欄位的值永遠不會傳回瀏覽器：表單顯示預留位置內容，原樣儲存預留位置內容會保留已儲存的值。

帶有瀏覽器套件的外掛可以用自己的元件取代產生的表單，參見[瀏覽器套件](./webapp.md#settings-panel)。
