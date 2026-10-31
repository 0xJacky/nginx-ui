---
outline: [2, 3]
---

# 插件开发概览

插件可以在不修改 Nginx UI 本身的情况下扩展它的功能：为证书签发添加 DNS 服务商、添加通知渠道和备份目的地、在网页界面中添加页面和面板、提供 nginx 配置模板等等。插件可以从插件目录安装，也可以上传插件包或从服务器上的目录安装，并且可以随时启用、禁用和更新。

本指南面向插件开发者。它说明了插件必须提供什么、可以依赖什么，因此插件可以用任何语言编写。

## 插件的组成 {#what-a-plugin-is-made-of}

插件是一个在根目录包含 `plugin.json` [清单文件](./manifest.md)的插件包。清单声明以下三个部分中的一个或多个：

| 部分 | 内容 | 适用场景 |
| --- | --- | --- |
| `server` | 由 Nginx UI 启动并与之通信的可执行文件。 | 插件需要在服务器上运行代码：调用服务商 API、提供 HTTP 接口、响应能力调用。 |
| `webapp` | 加载到 Nginx UI 网页界面中的 JavaScript 包，或在框架中显示的静态 HTML 页面。 | 插件要在界面中添加页面、面板、表单字段或列。 |
| `content` | nginx 配置模板和翻译文件。 | 插件只提供数据。这类插件完全没有进程。 |

一个插件可以同时包含这三个部分。例如官方 DNS-01 插件既有与 DNS 服务商通信的服务端进程，也有把字段添加到证书表单中的浏览器包。

## 服务端进程如何工作 {#how-the-server-process-works}

Nginx UI 启动可执行文件，并通过进程的标准输入和标准输出与它交换 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) 消息，每行一个 JSON 对象。双方都可以发送请求：

- Nginx UI 调用插件来驱动它的[生命周期](./lifecycle.md)（启动、配置、健康检查、停止），并使用它的[能力](#capabilities)。
- 插件通过[宿主 API](./host-api.md) 调用 Nginx UI：键值存储、插件设置、通知、定时任务、已保存的凭据。

插件还可以通过 gRPC 提供同样的调用以提升速度，而访问日志推送必须使用 gRPC。[通信协议](./protocol.md)一页介绍了这两种传输方式。

插件进程以与 Nginx UI 相同的操作系统权限运行。[权限](./permissions.md)决定插件可以使用 Nginx UI 的哪些功能，而不是它在机器上能做什么，所以安装插件就意味着信任它的发布者。[签名](./signing.md)让安装插件的人知道是谁发布了它。

## 能力 {#capabilities}

能力是 Nginx UI 在自身功能之外交给插件提供的功能。插件在清单中声明它实现的能力，每当用到该功能时，Nginx UI 就会调用它。

| 能力 | 插件做什么 | 页面 |
| --- | --- | --- |
| `dns01` | 通过 DNS 服务商发布 ACME DNS-01 验证所需的 TXT 记录。 | [DNS-01](./capabilities/dns01.md) |
| `http` | 在 Nginx UI 的身份验证之后提供 HTTP API 或页面。 | [HTTP 接口](./http.md) |
| `notify` | 通过聊天服务、推送网关或告警系统发送 Nginx UI 的通知。 | [通知渠道](./capabilities/notify.md) |
| `probe` | 添加网站健康检查的类型。 | [健康检查](./capabilities/probe.md) |
| `mcp` | 为 AI 助手向 Nginx UI 的 MCP 服务器添加工具。 | [MCP 工具](./capabilities/mcp.md) |
| `storage` | 把备份保存到 Nginx UI 自身无法访问的地方。 | [存储](./capabilities/storage.md) |
| `cert.deploy` | 把签发的证书推送到 CDN、负载均衡和其他服务器。 | [证书部署](./capabilities/cert-deploy.md) |
| `security.blocklist` | 获取需要拒绝访问的地址列表。 | [封禁列表](./capabilities/blocklist.md) |
| `upstream.discovery` | 把一个服务解析成 nginx upstream 的服务器。 | [上游发现](./capabilities/discovery.md) |
| `log.sink` | 在 nginx 写入访问日志时实时接收日志行。 | [访问日志推送](./capabilities/log-sink.md) |

模板和翻译文件不需要能力，它们属于[内容](./capabilities/content.md)。

## SDK {#sdks}

协议是通过管道传输的普通 JSON，因此任何语言都可以使用。SDK 替你处理协议细节：

| SDK | 语言 | 覆盖范围 |
| --- | --- | --- |
| [plugin-sdk-go](https://github.com/nginxui/plugin-sdk-go) | Go | 服务端进程：协议、生命周期、所有能力、gRPC、HTTP。 |
| [plugin-sdk-rust](https://github.com/nginxui/plugin-sdk-rust) | Rust | 服务端进程：协议、生命周期、所有能力、gRPC、HTTP。 |
| [@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) | TypeScript | 浏览器包：运行时类型、Vite 预设、静态页面辅助工具。 |

其他语言的插件直接实现协议即可。[plugin-spec](https://github.com/nginxui/plugin-spec) 仓库存放了每个 SDK 所依据的机器可读定义：所有消息的 protobuf 定义、清单和插件目录的 JSON Schema、请求与响应样例，以及一个没有任何依赖的 Python 插件示例。

## 下一步 {#where-to-go-next}

- [快速上手](./quick-start.md)：创建、构建并安装第一个插件。
- [开发与调试](./development.md)：介绍发布前检查插件的工具。
- [打包](./packaging.md)、[签名与信任](./signing.md)和[插件目录](./catalog.md)：介绍如何发布。
