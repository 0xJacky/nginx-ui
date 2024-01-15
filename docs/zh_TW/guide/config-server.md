# 服務端

Nginx UI 配置的服務端部分涉及控制 Nginx UI 服務器的各種設置。在頁面中，我們將討論可用的選項、它們的預設值以及它們的目的。

## HttpHost
- 類型: `string`
- 預設值：`0.0.0.0`

Nginx UI 服務器監聽的主機名。此選項用於配置 Nginx UI 服務器監聽傳入 HTTP 請求的主機名。 更改預設主機名可能有助於提升安全性。

## HttpPort

- 類型：`int`
- 預設值：`9000`

Nginx UI 服務器監聽端口。此選項用於配置 Nginx UI 服務器監聽傳入 HTTP 請求的端口。更改預設端口對於避免端口衝突或增強安全性可能很有用。

## RunMode

- 類型：`string`
- 支援的值：`release`，`debug`
- 預設值：`debug`

此選項用於配置 Nginx UI 服務器的運行模式，主要影響日誌打印的級別。

Nginx UI 的日誌分為 6 個級別，分別為 `Debug`、`Info`、`Warn`、`Error`、`Panic` 和 `Fatal`，這些日誌級別按照嚴重程度遞增，

當使用 `debug` 模式時，Nginx UI 將在控制台打印 SQL 及其執行的時間和調用者，`Debug` 級別或更高等級的日誌也會被打印。

當使用 `release` 模式時，Nginx UI 將不會在控制台打印 SQL 的執行時間和調用者， 只有 `Info` 級別或更高等級的日誌才會被打印。

## JwtSecret
- 類型：`string`

此選項用於配置 Nginx UI 服務器用於生成 JWT 的密鑰。

JWT 是一種用於驗證用戶身份的標準，它可以在用戶登錄後生成一個 token，然後在後續的請求中使用該 token 來驗證用戶身份。

如果您使用一鍵安裝腳本來部署 Nginx UI，腳本將會生成一個 UUID 值並將它設置為此選項的值。

## HTTPChallengePort

- 類型：`int`
- 預設值：`9180`

在獲取 Let's Encrypt 證書時，此選項用於在 HTTP01

挑戰模式中設置後端監聽端口。HTTP01 挑戰是 Let's Encrypt 用於驗證您控制請求證書的域的域驗證方法。

## Email
- 類型：`string`

在獲取 Let's Encrypt 證書時，此選項用於設置您的電子郵件地址。Let's Encrypt 會將您的電子郵件地址用於通知您證書的到期時間。

## Database

- 類型：`string`
- 預設值：`database`

此選項用於設置 Nginx UI 用於存儲其數據的 sqlite 數據庫的名稱。

## StartCmd

- 類型：`string`
- 預設值：`login`

此選項用於設置 Web 終端的啟動命令。

::: warning 警告
出於安全原因，我們將啟動命令設置為 `login`，因此您必須通過 Linux 的預設身份驗證方法登錄。如果您不想每次訪問 Web
終端時都輸入用戶名和密碼進行驗證，請將其設置為 `bash` 或 `zsh`（如果已安裝）。
:::

## PageSize

- 類型：`int`
- 預設值：`10`

此選項用於設置 Nginx UI 中列表分頁的頁面大小。調整頁面大小有助於更有效地管理大量數據，但是過大的數量可能會增加服務器的壓力。

## CADir

- 類型：`string`

在申請 Let's Encrypt 證書時，我們使用 Let's Encrypt 的預設 CA 地址。如果您需要調試或從其他提供商獲取證書，您可以將 CADir
設置為他們的地址。

::: tip 提示
請注意，CADir 提供的地址需要符合 `RFC 8555` 標準。
:::

## GithubProxy

- 類型：`string`
- 建議：`https://mirror.ghproxy.com/`

對於可能在從 Github 下載資源時遇到困難的用戶（如在中國大陸），此選項允許他們為 github.com 設置代理，以提高可訪問性。
