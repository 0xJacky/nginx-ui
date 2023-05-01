# 服务端

Nginx UI 配置的服务端部分涉及控制 Nginx UI 服务器的各种设置。在本节中，我们将讨论可用的选项、它们的默认值以及它们的目的。

## HttpPort

- 类型：`int`
- 默认值：`9000`

Nginx UI 服务器监听端口。此选项用于配置 Nginx UI 服务器监听传入 HTTP 请求的端口。更改默认端口对于避免端口冲突或增强安全性可能很有用。

## RunMode

- 类型：`string`
- 支持的值：`release`，`debug`

::: tip
目前，我们尚未适应此选项，在使用方面，`release` 和 `debug` 之间不会有显著差异。
:::

## HTTPChallengePort

- 类型：`int`
- 默认值：`9180`

在获取 Let's Encrypt 证书时，此选项用于在 HTTP01 挑战模式中设置后端监听端口。HTTP01 挑战是 Let's Encrypt
用于验证您控制请求证书的域的域验证方法。

## Database

- 类型：`string`
- 默认值：`database`

此选项用于设置 Nginx UI 用于存储其数据的 sqlite 数据库的名称。

## StartCmd

- 类型：`string`
- 默认值：`login`

此选项用于设置 Web 终端的启动命令。

::: warning
出于安全原因，我们将启动命令设置为 `login`，因此您必须通过 Linux 的默认身份验证方法登录。如果您不想每次访问 Web
终端时都输入用户名和密码进行验证，请将其设置为 `bash` 或 `zsh`（如果已安装）。
:::

## PageSize

- 类型：`int`
- 默认值：10

此选项用于设置 Nginx UI 中列表分页的页面大小。调整页面大小有助于更有效地管理大量数据,但是过大的数量可能会增加服务器的压力。

## CADir

- 类型：`string`

在申请 Let's Encrypt 证书时，我们使用 Let's Encrypt 的默认 CA 地址。如果您需要调试或从其他提供商获取证书，您可以将 CADir
设置为他们的地址。

::: tip
请注意，CADir 提供的地址需要符合 `RFC 8555` 标准。
:::

## GithubProxy

- 类型：`string`
- 建议：`https://ghproxy.com/`

对于可能在从 Github 下载资源时遇到困难的中国大陆用户，此选项允许他们为 github.com 设置代理，以提高可访问性。
