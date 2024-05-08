# 服务端

Nginx UI 配置的服务端部分涉及控制 Nginx UI 服务器的各种设置。在页面中，我们将讨论可用的选项、它们的默认值以及它们的目的。

## HttpHost
- 类型: `string`
- 默认值：`0.0.0.0`

Nginx UI 服务器监听的主机名。此选项用于配置 Nginx UI 服务器监听传入 HTTP 请求的主机名。 更改默认主机名可能有助于提升安全性。

## HttpPort

- 类型：`int`
- 默认值：`9000`

Nginx UI 服务器监听端口。此选项用于配置 Nginx UI 服务器监听传入 HTTP 请求的端口。更改默认端口对于避免端口冲突或增强安全性可能很有用。

## RunMode

- 类型：`string`
- 支持的值：`release`，`debug`
- 默认值：`debug`

此选项用于配置 Nginx UI 服务器的运行模式，主要影响日志打印的级别。

Nginx UI 的日志分为 6 个级别，分别为 `Debug`、`Info`、`Warn`、`Error`、`Panic` 和 `Fatal`，这些日志级别按照严重程度递增，

当使用 `debug` 模式时，Nginx UI 将在控制台打印 SQL 及其执行的时间和调用者，`Debug` 级别或更高等级的日志也会被打印。

当使用 `release` 模式时，Nginx UI 将不会在控制台打印 SQL 的执行时间和调用者， 只有 `Info` 级别或更高等级的日志才会被打印。

## JwtSecret
- 类型：`string`

此选项用于配置 Nginx UI 服务器用于生成 JWT 的密钥。

JWT 是一种用于验证用户身份的标准，它可以在用户登录后生成一个 token，然后在后续的请求中使用该 token 来验证用户身份。

如果您使用一键安装脚本来部署 Nginx UI，脚本将会生成一个 UUID 值并将它设置为此选项的值。

## HTTPChallengePort

- 类型：`int`
- 默认值：`9180`

在获取 Let's Encrypt 证书时，此选项用于在 HTTP01 挑战模式中设置后端监听端口。HTTP01 挑战是 Let's Encrypt
用于验证您控制请求证书的域的域验证方法。

## Email
- 类型：`string`

在获取 Let's Encrypt 证书时，此选项用于设置您的电子邮件地址。Let's Encrypt 会将您的电子邮件地址用于通知您证书的到期时间。

## Database

- 类型：`string`
- 默认值：`database`

此选项用于设置 Nginx UI 用于存储其数据的 sqlite 数据库的名称。

## StartCmd

- 类型：`string`
- 默认值：`login`

此选项用于设置 Web 终端的启动命令。

::: warning 警告
出于安全原因，我们将启动命令设置为 `login`，因此您必须通过 Linux 的默认身份验证方法登录。如果您不想每次访问 Web
终端时都输入用户名和密码进行验证，请将其设置为 `bash` 或 `zsh`（如果已安装）。
:::

## PageSize

- 类型：`int`
- 默认值：`10`

此选项用于设置 Nginx UI 中列表分页的页面大小。调整页面大小有助于更有效地管理大量数据，但是过大的数量可能会增加服务器的压力。

## CADir

- 类型：`string`

在申请 Let's Encrypt 证书时，我们使用 Let's Encrypt 的默认 CA 地址。如果您需要调试或从其他提供商获取证书，您可以将 CADir
设置为他们的地址。

::: tip 提示
请注意，CADir 提供的地址需要符合 `RFC 8555` 标准。
:::

## GithubProxy

- 类型：`string`
- 建议：`https://mirror.ghproxy.com/`

对于可能在从 Github 下载资源时遇到困难的用户（如在中国大陆），此选项允许他们为 github.com 设置代理，以提高可访问性。

## CertRenewalInterval

- 版本：`>= v2.0.0-beta.22`
- 类型：`int`
- 默认值: `7`

此选项用于设置 Let's Encrypt 证书的自动续签间隔。默认情况下，Nginx UI 每隔 7 天会自动续签证书。

## RecursiveNameservers

- 版本：`>= v2.0.0-beta.22`
- 类型: `[]string`
- 示例: `8.8.8.8:53,1.1.1.1:53`

此选项用于设置 Nginx UI 在申请证书的 DNS 挑战步骤所使用的递归域名服务器。在不配置此项目的情况下，Nginx UI 使用操作系统的域名服务器设置。

## SkipInstallation

- 版本：`>= v2.0.0-beta.23`
- 类型：`bool`
- 默认值：`false`

通过将此选项设置为 `true`，您可以跳过 Nginx UI 服务器的安装。
当您希望使用相同的配置文件或环境变量将 Nginx UI 部署到多个服务器时，这非常有用。

默认情况下，如果您启用了跳过安装模式，而没有在服务器部分设置 `JWTSecret` 和 `NodeSecret` 选项，
Nginx UI 将为这两个选项生成一个随机的 UUID 值。

此外，如果您也没有在服务器部分设置 `Email` 选项，
Nginx UI 将不会创建系统初始的 acme 用户，这意味着您无法在此服务器上申请 SSL 证书。

## Name

- 版本：`>= v2.0.0-beta.23`
- 类型：`string`

使用此选项自定义本地服务器的名称，以在环境指示器中显示。
