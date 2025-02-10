# Cert

## CADir
- 类型: `string`
- 版本：`>= v2.0.0-beta.37`

在申请 Let's Encrypt 证书时，我们使用 Let's Encrypt 的默认 CA 地址。
如果您需要调试或从其他提供商获取证书，您可以将 CADir 设置为他们的地址。

::: tip 提示
请注意，CADir 提供的地址需要符合 `RFC 8555` 标准。
:::

## RecursiveNameservers

- 版本：`>= v2.0.0-beta.37`
- 类型: `[]string`
- 示例: `8.8.8.8:53,1.1.1.1:53`

此选项用于设置 PrimeWaf 在申请证书的 DNS 挑战步骤所使用的递归域名服务器。在不配置此项目的情况下，PrimeWaf 使用操作系统的域名服务器设置。

## CertRenewalInterval

- 版本：`>= v2.0.0-beta.37`
- 类型: `int`
- 默认值: `7`

此选项用于设置 Let's Encrypt 证书的自动续签间隔。默认情况下，PrimeWaf 每隔 7 天会自动续签证书。

## HTTPChallengePort

- 版本：`>= v2.0.0-beta.37`
- 类型: `int`
- 默认值: `9180`

在获取 Let's Encrypt 证书时，此选项用于在 HTTP01 挑战模式中设置后端监听端口。
HTTP01 挑战是 Let's Encrypt 用于验证您控制请求证书的域的域验证方法。
