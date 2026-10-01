---
outline: [2, 3]
---

# 命名規則

外掛選用的一些名稱會與其他所有外掛共用，因此需要遵循本頁的規則。這些名稱一旦發佈，都不應再變更。

## 外掛 ID {#plugin-ids}

外掛 ID 是一個反向網域名稱：兩段或多段以點分隔的小寫字母、數字和連字號，最長 64 個字元，最重要的一段在前：

```text
com.example.mydns
io.github.example.mydns
```

- 使用你擁有的網域名稱，並將其反轉。
- 如果沒有自己的網域名稱，請使用 `io.github.<owner>.<name>`，其中 `<owner>` 是你的 GitHub 使用者名稱或組織名稱（小寫）。每個 GitHub 使用者都已經擁有這個命名空間。
- `com.nginxui.*` 保留給 Nginx UI 專案的外掛，官方外掛目錄會拒絕其他人使用它。無論外掛套件來自哪個外掛目錄或檔案，Nginx UI 只安裝由官方金鑰簽署的此類外掛。開發者模式下不受此限制，以便建置官方外掛。

::: warning 注意
ID 永久識別一個外掛：它的設定、資料、憑證和目錄項目都與 ID 關聯。改名就意味著發佈一個新外掛。
:::

## 服務商與類型代碼 {#provider-and-kind-codes}

向 Nginx UI 表單加入選項的能力使用 `code` 識別每個選項：

| 能力 | 代碼識別的對象 |
| --- | --- |
| `dns01` | DNS 服務商 |
| `notify` | 通知通道 |
| `probe` | 健康檢查類型 |
| `storage` | 儲存後端 |
| `cert.deploy` | 部署目標類型 |
| `security.blocklist` | 封鎖清單來源類型 |
| `upstream.discovery` | 探索提供者 |

代碼符合 `^[a-z0-9-]{2,32}$`，且在一個清單中只出現一次。代碼在同一能力的所有已安裝外掛之間共用：憑證、通知通道或備份工作只儲存代碼，使用時 Nginx UI 再找到擁有該代碼的外掛。因此：

- **用服務商或技術命名，而不是用外掛命名**：用 `cloudflare`，不要用 `mydns-cloudflare`。絕不要為代碼加上外掛 ID 前綴。
- **避免使用其他外掛已經在用的代碼**，尤其是官方 DNS-01 外掛的服務商代碼，除非你的外掛就是要取代那個實作。如果另一個已啟用的外掛已經註冊了某個代碼，Nginx UI 可能會拒絕啟用你的外掛。
- **絕不要重新命名已發佈的代碼。** 使用舊代碼設定的所有內容都會失效。

多個已啟用的外掛宣告同一個代碼時，Nginx UI 使用外掛 ID 最小的那一個。外掛的代碼永遠不會與 Nginx UI 自己的通道、檢查或儲存衝突，它們是分開管理的。

## MCP 工具名稱 {#mcp-tool-names}

MCP 工具名稱符合 `^[a-z0-9][a-z0-9_-]{0,47}$`，且在外掛內唯一。Nginx UI 發佈工具時會在前面加上外掛 ID（點取代為底線），中間用兩個底線連接：

```text
io.github.example.cdn + purge_cache  →  io_github_example_cdn__purge_cache
```

有些模型 API 只接受 64 個字元，因此請讓發佈後的名稱不超過 64 個字元。不要重新命名已發佈的工具，因為用戶端和提示詞會依名稱參照它。

## 設定鍵與插槽名稱 {#settings-keys-and-slot-names}

設定鍵只屬於一個外掛，所以使用 `timeout` 這樣的短鍵沒有問題。

外掛為自己定義的插槽（不屬於 [Nginx UI 插槽](./slots.md)的插槽）應以外掛 ID 作為前綴，例如 `io.github.example.mydns:extra-panel`，以免與 Nginx UI 或其他外掛將來的插槽衝突。

## 平台鍵 {#platform-keys}

平台鍵的格式是 `<goos>-<goarch>`：Go 工具鏈的 `GOOS` 和 `GOARCH` 值用連字號連接，與 `go tool dist list` 的輸出一致，例如 `linux-amd64`、`linux-arm64`、`darwin-arm64`、`windows-amd64`。它們用於 `server.executables`、外掛目錄中版本的 `platforms` 和 `downloads`，以及外掛套件檔名。`any` 在外掛目錄中表示所有平台，但不能用於檔名。

## 外掛套件檔名 {#package-file-names}

```text
<id>-<version>.tar.gz                  通用外掛套件
<id>-<version>-<goos>-<goarch>.tar.gz  單一平台的外掛套件
```

例如 `com.nginxui.dns01-1.0.0.tar.gz` 和 `com.nginxui.dns01-1.0.0-linux-arm64.tar.gz`。只有當檔名最後兩段（以連字號分隔）是已知的 GOOS 和 GOARCH 時，才會被視為單一平台的外掛套件，因此不要發佈預發佈部分以這樣一對結尾的版本，例如 `1.0.0-linux-amd64`。

Nginx UI 從不依賴檔名：它需要的一切都來自套件內的清單，上傳的外掛套件可以使用任何名稱。
