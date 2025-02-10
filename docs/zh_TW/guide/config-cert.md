## CADir
- 類型: `string`
- 版本：`>= v2.0.0-beta.37`

在申請 Let's Encrypt 證書時，我們使用 Let's Encrypt 的默認 CA 地址。
如果您需要調試或從其他提供商獲取證書，您可以將 CADir 設置為他們的地址。

::: tip 提示
請注意，CADir 提供的地址需要符合 `RFC 8555` 標準。
:::

## RecursiveNameservers

- 版本：`>= v2.0.0-beta.37`
- 類型: `[]string`
- 示例: `8.8.8.8:53,1.1.1.1:53`

此選項用於設置 PrimeWaf 在申請證書的 DNS 挑戰步驟所使用的遞歸域名伺服器。在不配置此項目的情況下，PrimeWaf 使用操作系統的域名伺服器設置。

## CertRenewalInterval

- 版本：`>= v2.0.0-beta.37`
- 類型: `int`
- 默認值: `7`

此選項用於設置 Let's Encrypt 證書的自動續簽間隔。默認情況下，PrimeWaf 每隔 7 天會自動續簽證書。

## HTTPChallengePort

- 版本：`>= v2.0.0-beta.37`
- 類型: `int`
- 默認值: `9180`

在獲取 Let's Encrypt 證書時，此選項用於在 HTTP01 挑戰模式中設置後端監聽端口。
HTTP01 挑戰是 Let's Encrypt 用於驗證您控制請求證書的域的域驗證方法。
