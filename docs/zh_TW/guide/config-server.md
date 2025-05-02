# Server

Nginx UI 設定的服務端部分涉及控制 Nginx UI 伺服器的各種設定。在頁面中，我們將討論可用的選項、它們的預設值以及它們的目的。

## Host
- 類型：`string`
- 版本：`>= v2.0.0-beta.37`
- 預設值：`0.0.0.0`

Nginx UI 伺服器監聽的主機名稱。此選項用於設定 Nginx UI 伺服器監聽傳入 HTTP 請求的主機名稱。更改預設主機名稱可能有助於提升安全性。

## Port
- 類型：`uint`
- 版本：`>= v2.0.0-beta.37`
- 預設值：`9000`

此選項用於設定 Nginx UI 伺服器監聽傳入 HTTP 請求的連接埠。更改預設連接埠對於避免連接埠衝突或增強安全性可能很有用。

## RunMode

- 類型：`string`
- 支援的值：`release`，`debug`
- 預設值：`debug`

此選項用於設定 Nginx UI 伺服器的執行模式，主要影響日誌輸出的級別。

Nginx UI 的日誌分為 6 個級別，分別為 `Debug`、`Info`、`Warn`、`Error`、`Panic` 和 `Fatal`，這些日誌級別按照嚴重程度遞增。

當使用 `debug` 模式時，Nginx UI 將在控制檯列印 SQL 及其執行的時間和呼叫者，`Debug` 級別或更高階別的日誌也會被列印。

當使用 `release` 模式時，Nginx UI 將不會在控制檯列印 SQL 的執行時間和呼叫者，只有 `Info` 級別或更高階別的日誌才會被列印。

## HttpHost
- 類型：`string`
- 預設值：`0.0.0.0`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Host` 取代。
:::

Nginx UI 伺服器監聽的主機名稱。此選項用於設定 Nginx UI 伺服器監聽傳入 HTTP 請求的主機名稱。更改預設主機名稱可能有助於提升安全性。

## HttpPort
- 類型：`int`
- 預設值：`9000`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Port` 取代。
:::

此選項用於設定 Nginx UI 伺服器監聽傳入 HTTP 請求的連接埠。更改預設連接埠對於避免連接埠衝突或增強安全性可能很有用。

## JwtSecret
- 類型：`string`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `App.JwtSecret` 取代。
:::

此選項用於設定 Nginx UI 伺服器用於生成 JWT 的金鑰。

JWT 是一種用於驗證使用者身份的標準，它可以在使用者登入後生成一個 token，然後在後續的請求中使用該 token 來驗證使用者身份。

如果您使用一鍵安裝指令碼來部署 Nginx UI，指令碼將會生成一個 UUID 值並將它設定為此選項的值。

## NodeSecret
- 類型：`string`
- 版本：`>= v2.0.0-beta.24, <= 2.0.0-beta.36`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Node.Secret` 取代。
:::

此金鑰用於驗證 Nginx UI 伺服器之間的通訊。
此外，您可以使用此金鑰在不使用密碼的情況下存取 Nginx UI API。

## HTTPChallengePort

- 類型：`int`
- 預設值：`9180`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Cert.HTTPChallengePort` 取代。
:::

在取得 Let's Encrypt 證書時，此選項用於在 HTTP01 挑戰模式中設定後端監聽連接埠。HTTP01 挑戰是 Let's Encrypt 用於驗證您控制請求證書的域的域驗證方法。

## Email
- 類型：`string`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Cert.Email` 取代。
:::

在取得 Let's Encrypt 證書時，此選項用於設定您的電子郵件地址。Let's Encrypt 會將您的電子郵件地址用於通知您證書的到期時間。

## Database

- 類型：`string`
- 預設值：`database`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Database.Name` 取代。
:::

此選項用於設定 Nginx UI 用於儲存其資料的 sqlite 資料庫的名稱。

## StartCmd

- 類型：`string`
- 預設值：`login`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Terminal.StartCmd` 取代。
:::

此選項用於設定 Web 終端的啟動命令。

::: warning 警告
出於安全原因，我們將啟動命令設定為 `login`，因此您必須透過 Linux 的預設身份驗證方法登入。如果您不想每次存取 Web 終端時都輸入使用者名稱和密碼進行驗證，請將其設定為 `bash` 或 `zsh`（如果已安裝）。
:::

## PageSize

- 類型：`int`
- 預設值：`10`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `App.PageSize` 取代。
:::

此選項用於設定 Nginx UI 中列表分頁的頁面大小。調整頁面大小有助於更有效地管理大量資料，但是過大的數量可能會增加伺服器的壓力。

## CADir

- 類型：`string`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Cert.CADir` 取代。
:::

在申請 Let's Encrypt 證書時，我們使用 Let's Encrypt 的預設 CA 地址。如果您需要除錯或從其他供應商取得證書，您可以將 CADir 設定為他們的地址。

::: tip 提示
請注意，CADir 提供的地址需要符合 `RFC 8555` 標準。
:::

## GithubProxy

- 類型：`string`
- 建議：`https://cloud.nginxui.com/`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Http.GithubProxy` 取代。
:::

對於可能在從 Github 下載資源時遇到困難的使用者（如在中國大陸），此選項允許他們為 github.com 設定代理，以提高可存取性。

## CertRenewalInterval

- 版本：`>= v2.0.0-beta.22, <= 2.0.0-beta.36`
- 類型：`int`
- 預設值：`7`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Cert.CertRenewalInterval` 取代。
:::

此選項用於設定 Let's Encrypt 證書的自動續簽間隔。預設情況下，Nginx UI 每隔 7 天會自動續簽證書。

## RecursiveNameservers

- 版本：`>= v2.0.0-beta.22, <= 2.0.0-beta.36`
- 類型：`[]string`
- 範例：`8.8.8.8:53,1.1.1.1:53`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用

`Cert.RecursiveNameservers` 取代。
:::

此選項用於設定 Nginx UI 在申請證書的 DNS 挑戰步驟中所使用的遞迴域名伺服器。在不設定此專案的情況下，Nginx UI 使用作業系統的域名伺服器設定。

## SkipInstallation

- 版本：`>= v2.0.0-beta.23, <= 2.0.0-beta.36`
- 類型：`bool`
- 預設值：`false`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Node.SkipInstallation` 取代。
:::

透過將此選項設定為 `true`，您可以跳過 Nginx UI 伺服器的安裝。
當您希望使用相同的設定檔或環境變數將 Nginx UI 部署到多個伺服器時，這非常有用。

預設情況下，如果您啟用了跳過安裝模式，而沒有在伺服器部分設定 `JWTSecret` 和 `NodeSecret` 選項，Nginx UI 將為這兩個選項生成一個隨機的 UUID 值。

此外，如果您也沒有在伺服器部分設定 `Email` 選項，Nginx UI 將不會建立系統初始的 acme 使用者，這意味著您無法在此伺服器上申請 SSL 證書。

## Name

- 版本：`>= v2.0.0-beta.23, <= 2.0.0-beta.36`
- 類型：`string`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Http.InsecureSkipVerify` 取代。
:::

使用此選項自定義本機伺服器的名稱，以在環境指示器中顯示。

## InsecureSkipVerify

- 版本：`>= v2.0.0-beta.30, <= 2.0.0-beta.36`
- 類型：`bool`

::: warning 警告
已在 `v2.0.0-beta.37` 中廢棄，請使用 `Http.InsecureSkipVerify` 取代。
:::

此選項用於設定 Nginx UI 伺服器在與其他伺服器建立 TLS 連接時是否跳過證書驗證。
