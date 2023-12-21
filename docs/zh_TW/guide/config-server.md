# 服務端

Nginx UI 配置的服務端部分涉及控制 Nginx UI 服務的各種設定。在本節中，我們將討論可用的選項、它們的預設值以及它們的目的。

## HttpPort

- 型別：`int`
- 預設值：`9000`

Nginx UI 監聽埠。此選項用於配置 Nginx UI 伺服器監聽傳入 HTTP 請求的埠。更改預設埠對於避免埠衝突或增強安全性可能很有用。

## RunMode

- 型別：`string`
- 支援的值：`release`，`debug`

::: tip 提示
目前，此選項尚無影響。在使用方面，`release` 和 `debug` 之間不會有顯著差異。
:::

## HTTPChallengePort

- 型別：`int`
- 預設值：`9180`

在獲取 Let's Encrypt 憑證時，此選項用於在 HTTP01 挑戰模式中設定後端監聽埠。HTTP01 挑戰是 Let's Encrypt
用於驗證您控制請求憑證的域的域驗證方法。

## Database

- 型別：`string`
- 預設值：`database`

此選項用於設定 Nginx UI 用於儲存其資料的 sqlite 資料庫的名稱。

## StartCmd

- 型別：`string`
- 預設值：`login`

此選項用於設定 Web 終端的啟動命令。

::: warning 警告
出於安全原因，我們將啟動命令設定為 `login`，因此您必須透過 Linux 的預設身份驗證方法登入。如果您不想每次訪問 Web
終端時都輸入使用者名稱和密碼進行驗證，請將其設定為 `bash` 或 `zsh`（如果已安裝）。
:::

## PageSize

- 型別：`int`
- 預設值：10

此選項用於設定 Nginx UI 中列表分頁的頁面大小。調整頁面大小有助於更有效地管理大量資料,但是過大的數量可能會增加伺服器的壓力。

## CADir

- 型別：`string`

在申請 Let's Encrypt 憑證時，我們使用 Let's Encrypt 的預設 CA 位址。如果您需要除錯或從其他提供商獲取憑證，您可以將 CADir
設定為他們的位址。

::: tip 提示
請注意，CADir 提供的位址需要符合 `RFC 8555` 標準。
:::

## GithubProxy

- 型別：`string`
- 建議：`https://mirror.ghproxy.com/`

對於可能在從 Github 下載資源時遇到困難的使用者（如在中國大陸），此選項允許他們為 github.com 設定代理，以提高可訪問性。
