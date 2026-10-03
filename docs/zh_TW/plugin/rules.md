---
outline: [2, 3]
---

# 檢查規則

`nginx-ui plugin lint` 和 `nginx-ui plugin conformance` 回報的每個問題都附帶它違反的規則名稱。本頁解釋每一條規則，並連結到指南中的相關內容。

- <Badge type="info" text="lint" /> 規則根據外掛目錄或外掛套件中的檔案檢查。<Badge type="danger" text="error" /> 會阻止外掛套件安裝；<Badge type="warning" text="warning" /> 指出安裝外掛的人會注意到的問題。
- <Badge type="tip" text="conformance" /> 規則透過執行外掛來檢查。每個測試案例的結果為通過、失敗，或在不適用時略過。

## 清單 {#manifest}

### manifest-json

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` 缺少，或不是單一 JSON 物件。參見[清單檔案](./manifest.md)。

### manifest-id

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`id` 缺少、超過 64 個字元，或不是 `io.github.example.mydns` 這樣以點分隔的小寫 ID。參見[命名規則](./naming.md#plugin-ids)。

### manifest-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`name` 缺少或為空。

### manifest-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`version` 缺少，或不是語意化版本。

### manifest-api-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`api_version` 缺少，或不是 Nginx UI 實作的協定世代。參見[版本與相容性](./versioning.md)。

### manifest-parts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

清單沒有宣告 `server`、`webapp` 和 `content` 中的任何一個，外掛什麼也不會安裝。

### manifest-icon

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`icon_path` 不是[安全的相對路徑](./packaging.md#safe-paths)。

### manifest-i18n

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`i18n` 的某個鍵不是 Nginx UI 介面的語言。參見[清單檔案](./manifest.md#translations)。

### manifest-requires

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`requires` 中某一項的外掛 ID 無效。參見[相依與衝突](./manifest.md#dependencies-and-conflicts)。

### manifest-conflicts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`conflicts` 中某一項不是有效的外掛 ID、是外掛自己的 ID、重複出現，或同時出現在 `requires` 中。參見[相依與衝突](./manifest.md#dependencies-and-conflicts)。

### manifest-permissions

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

未知權限為錯誤，重複列出的權限為警告。參見[權限與安全](./permissions.md#permissions)。

### manifest-permission-reasons

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`permission_reasons` 或 `i18n` 中它的翻譯裡，某個鍵不是 `permissions` 中的一項，或者原因超過 300 個字元。參見[說明權限用途](./permissions.md#explaining-permissions)。

### manifest-screenshots

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`screenshots` 超過 8 張；某個 `id` 缺少、格式不對或者重複；某個路徑缺少、不是安全的相對路徑、不是 PNG、JPEG 或 WebP 圖片，或者重複出現；某則說明超過 200 個字元；或者 `i18n` 的 `screenshot_captions` 中某個鍵不是螢幕截圖的 id。參見[螢幕截圖](./manifest.md#screenshots)。

### capability-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個能力名稱不是 Nginx UI 認識的能力。參見[清單檔案](./manifest.md#capabilities)。

### capability-duplicate

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個能力被列出了兩次。

### server-entry

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server` 既沒有 `executables` 也沒有 `command`。參見[伺服器端進程](./manifest.md#server-process)。

### server-command

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.command` 的第一個元素為空。

### server-lifecycle

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.lifecycle` 不是 `resident` 或 `on_demand`。

### server-idle-timeout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.idle_timeout_seconds` 為負數。

### server-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

可執行檔路徑，或包含分隔符號的 `command` 第一個元素不是安全的相對路徑時為錯誤；在執行檢查工具的機器上，`command` 指定的程式在 `PATH` 中找不到時為警告。

### server-resources

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.resources` 的某個值為負數。參見[資源](./manifest.md#resources)。

### settings-key

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個設定欄位沒有鍵，或兩個欄位使用了同一個鍵。參見[設定](./manifest.md#settings)。

### settings-type

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

欄位型別未知，或 `list` 的預設值不是字串陣列時為錯誤；預設值與型別不符時為警告。

### settings-options

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`select` 欄位沒有選項。

### event-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

訂閱了 `log.paths_changed` 卻沒有 `log.files` 權限，因此該事件永遠不會送達。參見[宿主 API](./host-api.md#events)。

### reserved-namespace

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

ID 使用了保留給 Nginx UI 專案的 `com.nginxui.*`。參見[命名規則](./naming.md#plugin-ids)。

### network-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

與服務商或目標通訊的能力沒有宣告 `network` 權限。參見[權限與安全](./permissions.md#permissions)。

### unused-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

請求了只有外掛未宣告的能力才會用到的權限，例如沒有 `mcp` 能力卻請求 `mcp`。

## 外掛套件 {#package}

### package-file-name

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

外掛套件檔名中的 ID 或版本與清單不一致。參見[命名規則](./naming.md#package-file-names)。

### package-layout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` 既不在封存檔根目錄，也不在一層目錄之下，或者封存檔無法讀取。參見[打包](./packaging.md#layout)。

### package-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個封存檔項目不是[安全的相對路徑](./packaging.md#safe-paths)。

### package-links

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

封存檔中包含符號連結或硬連結。

### package-escape

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個項目會被解壓縮到外掛目錄之外。

### package-entries

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

外掛套件的項目超過 10,000 個。參見[限制](./packaging.md#limits)。

### package-size

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

外掛套件中的檔案解壓縮後超過 256 MiB。

### package-docs

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

缺少 `README.md` 時為錯誤；缺少 `LICENSE`，或者 README 缺少應有的章節時為警告。參見[打包](./packaging.md#layout)。

### package-executables

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

`server.executables` 或 `server.command` 宣告的檔案缺少或不是一般檔案時為錯誤；缺少執行權限時為警告。參見[可執行檔](./packaging.md#executables)。

### package-platform

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

單一平台的外掛套件沒有在 `server.executables` 中恰好只宣告自己的平台。參見[單一平台的外掛套件](./packaging.md#per-platform-packages)。

## 簽章 {#signature}

### signature-sums

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` 不符合規定的格式。參見[手動簽章](./signing.md#signing-by-hand)。

### signature-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

`plugin.sums` 和 `plugin.sums.minisig` 只有一個，因此外掛套件被視為未簽章。

### signature-mismatch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` 與檔案不符：列出的檔案缺少或 SHA-256 不同，或者有檔案沒有列出。外掛套件在簽章後被修改過，永遠無法安裝。參見 [Nginx UI 如何檢查外掛套件](./signing.md#how-nginx-ui-checks-a-package)。

### signature-signer

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

簽章無法用檢查工具已知的金鑰驗證：專案的官方外掛簽章金鑰，或外掛套件中帶有有效憑證的合作夥伴金鑰。對於社群外掛這是預期的，因為它的金鑰只由外掛目錄或維運人員指定。參見[信任等級](./signing.md#trust-levels)。

### partner-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

`plugin.partner` 和 `plugin.partner.minisig` 只有一個，因此外掛套件沒有合作夥伴憑證。參見[合作夥伴外掛](./signing.md#partner-plugins)。

### partner-comment

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

合作夥伴憑證無法驗證：`plugin.partner` 不是公鑰，沒有官方外掛簽章金鑰為它簽章，或者可信註解的格式錯誤。

### partner-certificate

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

合作夥伴憑證已到期，或者 `plugin.sums.minisig` 不是由憑證指定的金鑰簽署的，因此憑證對外掛套件不起作用。

## 協定與生命週期 {#protocol-and-lifecycle}

### protocol-stderr

<Badge type="tip" text="conformance" />

外掛把日誌寫到標準錯誤。參見[通訊協定](./protocol.md#framing)。

### protocol-notification

<Badge type="tip" text="conformance" />

未知方法的通知不會得到回覆，且連線仍然可用。參見[訊息](./protocol.md#messages)。

### protocol-concurrency

<Badge type="tip" text="conformance" />

20 個並行的 `plugin.ping` 呼叫全部得到回覆。

### protocol-errors

<Badge type="tip" text="conformance" />

外掛沒有的方法回覆 `-32601`，格式錯誤的參數回覆 `-32602` 或 `-32000`，而不是停滯。參見[錯誤碼](./protocol.md#error-codes)。

### protocol-grpc

<Badge type="tip" text="conformance" />

外掛回報的 gRPC 端點接受連線並回覆 `plugin.ping`。參見 [gRPC 傳輸](./protocol.md#grpc-transport)。

### protocol-transports

<Badge type="tip" text="conformance" />

同一個呼叫透過標準輸入輸出和透過 gRPC 得到相同的結果或錯誤。兩種傳輸方式都測試過時才執行。

### lifecycle-handshake

<Badge type="tip" text="conformance" />

交握在時間限制內完成。參見[生命週期](./lifecycle.md#handshake)。

### lifecycle-api-version

<Badge type="tip" text="conformance" />

交握回報的協定世代與 Nginx UI 實作的一致。

### lifecycle-capabilities

<Badge type="tip" text="conformance" />

交握回報的能力與清單完全一致。

### lifecycle-ping

<Badge type="tip" text="conformance" />

`plugin.ping` 得到回覆。參見[健康檢查](./lifecycle.md#health-checks)。

### lifecycle-shutdown

<Badge type="tip" text="conformance" />

`plugin.shutdown` 和 `plugin.exit` 能及時停止進程。參見[停止](./lifecycle.md#stopping)。

## 網頁介面 {#web-interface}

### webapp-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`webapp.bundle_path` 或 `webapp.style_path` 不是安全的相對路徑。參見[瀏覽器套件](./webapp.md)。

### webapp-pages

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個頁面沒有 `path`，或其 `file` 不是安全的相對路徑。參見[靜態頁面](./pages.md)。

### webapp-chunks

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `webapp.chunks` 卻沒有 `bundle_path`，或者某個分塊的名稱或路徑無效、指向瀏覽器套件本身、與其他分塊共用路徑，或指向缺少的檔案。參見[分塊](./webapp.md#chunks)。

### webapp-bundle

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

瀏覽器套件檔案存在，並且是 Nginx UI 可以載入的指令碼。參見[建置](./webapp.md#building)。

### webapp-register

<Badge type="tip" text="conformance" />

瀏覽器套件用清單 ID 註冊自己。參見[註冊外掛](./webapp.md#registering-the-plugin)。

### webapp-chunk-files

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

分塊檔案為空或缺少時為錯誤；位於 Nginx UI 提供瀏覽器套件的目錄之外時為警告。

## DNS-01 {#dns-01}

### dns01-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `dns01` 能力卻沒有服務商。參見 [DNS-01](./capabilities/dns01.md#declaring-providers)。

### dns01-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服務商代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### dns01-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服務商沒有名稱。

### dns01-form

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服務商沒有 `form`，或者表單違反了[認證表單](./capabilities/dns01.md#credential-form)的規則：欄位沒有鍵或標籤、鍵重複、`group` 或 `unit` 未知、只有一種登入方式、方式名稱重複、方式列出了非認證欄位、`values` 的鍵使用不當、兩種方式完全相同，或者有多個推薦方式。

### dns01-present

<Badge type="tip" text="conformance" />

`dns01.present` 對第一個服務商有回覆。

### dns01-validate

<Badge type="tip" text="conformance" />

`dns01.validate` 對空設定的回覆符合預期。

### dns01-options

<Badge type="tip" text="conformance" />

`dns01.options` 有回覆，或回覆 `-32002`。

### dns01-check

<Badge type="tip" text="conformance" />

`dns01.check` 有回覆，或回覆 `-32002`。

## HTTP 介面 {#http-endpoints}

### http-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `http` 能力卻沒有 `http` 區塊，或者 `listen` 不是 `unix` 或 `rpc`。參見 [HTTP 介面](./http.md)。

## 設定表單 {#configuration-forms}

### configuration-fields

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`configuration` 表單中的某個欄位沒有鍵、鍵重複、型別未知或沒有 `display_name`。參見[設定表單](./capabilities/notify.md#configuration-form)。

## 通知通道 {#notification-channels}

### notify-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `notify` 能力卻沒有通道。參見[通知通道](./capabilities/notify.md)。

### notify-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

通道代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### notify-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

通道沒有名稱。

### notify-validate

<Badge type="tip" text="conformance" />

`notify.validate` 對空設定回覆指出必填欄位的 `-32003`，或者在通道沒有必填欄位時回覆 `{}`。測試從不呼叫 `notify.send`，因為它會存取真實服務。

## 健康檢查 {#health-checks}

### probe-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `probe` 能力卻沒有類型。參見[健康檢查](./capabilities/probe.md)。

### probe-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

類型代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### probe-kind

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

類型沒有名稱，或其設定表單無效。

### probe-check

<Badge type="tip" text="conformance" />

對無法連線的目標執行第一個類型的 `probe.check`，能夠及時回覆。

### probe-result

<Badge type="tip" text="conformance" />

回覆包含已知的 `status` 和非負的 `latency_ms`，或者是指出欄位的 `-32003`。

## MCP 工具 {#mcp-tools}

### mcp-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `mcp` 能力卻沒有工具，或沒有 `mcp` 權限。參見 [MCP 工具](./capabilities/mcp.md)。

### mcp-tool

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

工具名稱不符合 `^[a-z0-9][a-z0-9_-]{0,47}$`、被宣告了兩次，或者工具沒有描述。

### mcp-input-schema

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`input_schema` 的 `type` 不是 `object`。

### mcp-unknown-tool

<Badge type="tip" text="conformance" />

呼叫清單未宣告的工具會得到 `-32602`。測試不會呼叫任何已宣告的工具，因為它們可能改變狀態。

## 儲存 {#storage}

### storage-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `storage` 能力卻沒有後端。參見[儲存](./capabilities/storage.md)。

### storage-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

後端代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### storage-backend

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

後端沒有名稱，或其設定表單無效。

### storage-list

<Badge type="tip" text="conformance" />

使用空設定的 `storage.list` 能及時回覆，結果為指出必填欄位的 `-32003` 或一份清單。測試不會儲存、取回或刪除任何內容。

### storage-validate

<Badge type="tip" text="conformance" />

`storage.validate` 對空設定回覆指出必填欄位的 `-32003`，或回覆 `{}`。

## 憑證部署 {#certificate-deployment}

### deploy-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `cert.deploy` 能力卻沒有目標，或沒有 `cert.deploy` 權限。參見[憑證部署](./capabilities/cert-deploy.md)。

### deploy-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

目標代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### deploy-target

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

目標沒有名稱，或其設定表單無效。

### deploy-dry-run

<Badge type="tip" text="conformance" />

對一張臨時憑證的試執行推送能及時回覆。測試從不進行真正的推送。

### deploy-validate

<Badge type="tip" text="conformance" />

`deploy.validate` 對空設定回覆指出必填欄位的 `-32003`，或回覆 `{}`。

## 封鎖清單 {#blocklists}

### blocklist-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `security.blocklist` 能力卻沒有來源，或沒有 `network` 權限。參見[封鎖清單](./capabilities/blocklist.md)。

### blocklist-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

來源代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### blocklist-source

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

來源沒有名稱、設定表單無效，或者 `refresh_seconds` 介於 1 到 59 之間或為負數。

### blocklist-fetch

<Badge type="tip" text="conformance" />

使用空設定的 `blocklist.fetch` 能及時回覆，且項目中的位址都能解析。

### blocklist-errors

<Badge type="tip" text="conformance" />

需要設定的來源回覆指出欄位的 `-32003`，而不是空清單。

## 上游探索 {#upstream-discovery}

### discovery-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `upstream.discovery` 能力卻沒有提供者，或沒有 `network` 權限。參見[上游探索](./capabilities/discovery.md)。

### discovery-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

提供者代碼不符合 `^[a-z0-9-]{2,32}$`，或被宣告了兩次。

### discovery-provider

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

提供者沒有名稱，或其設定表單無效。

### discovery-resolve

<Badge type="tip" text="conformance" />

使用空設定的 `discovery.resolve` 能及時回覆，且目標的連接埠都在 1 到 65535 之間。

### discovery-errors

<Badge type="tip" text="conformance" />

需要設定的提供者或未知的服務回覆指出欄位的 `-32003`。

## 存取日誌串流 {#access-log-streaming}

### log-sink-permission

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

宣告了 `log.sink` 能力卻沒有 `log.read` 權限。參見[存取日誌串流](./capabilities/log-sink.md)。

### log-sink-block

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

有 `log_sink` 區塊卻沒有 `log.sink` 能力，該區塊不起作用。

### log-sink-batch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`batch_size` 不在 0 到 4096 之間，或者 `flush_interval_ms` 介於 1 到 49 之間或為負數。

### log-sink-formats

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

格式不是 `combined` 或 `raw`，或被列出了兩次。

### log-sink-transport

<Badge type="tip" text="conformance" />

透過標準輸入輸出傳送的 `log.push` 得到 `-32601`，並且外掛提供 gRPC。

### log-sink-push

<Badge type="tip" text="conformance" />

包含三個項目的串流能及時得到 `accepted` 為 3 的回覆。只測試標準輸入輸出時略過。

## 範本與翻譯 {#templates-and-translations}

### content-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`content.templates` 或 `content.locales` 不是安全的相對路徑。參見[範本與翻譯](./capabilities/content.md)。

### content-without-server

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

沒有 `server` 的清單宣告了 `capabilities`、`cron` 或 `events`，而它們都需要進程。

### content-templates

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

範本目錄缺少、不是目錄，或者 `conf/` 和 `block/` 中都沒有範本時為錯誤；有不是範本的項目時為警告。參見[範本](./capabilities/content.md#templates)。

### content-template

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

範本無法解析或無法用預設值渲染時為錯誤；沒有 `name` 時為警告。

### content-locales

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

翻譯目錄缺少、沒有 `.po` 檔案，或者有以 Nginx UI 不支援的語言命名的檔案時為錯誤；有其他項目時為警告。參見[翻譯](./capabilities/content.md#translations)。

### content-locale

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某個翻譯檔案無法解析。
