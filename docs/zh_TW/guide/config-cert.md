## CADir
- 類型：`string`
- 版本：`>= v2.0.0-beta.37`

在申請 Let's Encrypt 證書時，我們使用 Let's Encrypt 的預設 CA 地址。
如果您需要除錯或從其他供應商取得證書，您可以將 CADir 設定為他們的地址。

::: tip 提示
請注意，CADir 提供的地址需要符合 `RFC 8555` 標準。
:::

## RecursiveNameservers

- 版本：`>= v2.0.0-beta.37`
- 類型：`[]string`
- 範例：`8.8.8.8:53,1.1.1.1:53`

此選項用於設定 Nginx UI 在申請證書的 DNS 挑戰步驟所使用的遞迴域名伺服器。在不設定此專案的情況下，Nginx UI 使用作業系統的域名伺服器設定。

## CertRenewalInterval

- 版本：`>= v2.0.0-beta.37`
- 類型：`int`
- 預設值：`7`

此選項用於設定 Let's Encrypt 證書的自動續簽間隔。預設情況下，Nginx UI 每隔 7 天會自動續簽證書。

## HTTPChallengePort

- 版本：`>= v2.0.0-beta.37`
- 類型：`int`
- 預設值：`9180`

在取得 Let's Encrypt 證書時，此選項用於在 HTTP01 挑戰模式中設定後端監聽連接埠。
HTTP01 挑戰是 Let's Encrypt 用於驗證您控制請求證書的域的域驗證方法。
