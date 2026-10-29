# MCP 模块

## 简介

MCP（Model Context Protocol）是 Nginx UI 提供的一个特殊接口，允许 AI 代理与 Nginx UI 交互。通过 MCP，AI 模型可以访问和管理 Nginx 配置文件、执行 Nginx 相关操作（如重启、重载）以及获取 Nginx 运行状态。

## 功能概览

MCP 模块主要分为两大部分功能：

- [配置文件管理](./mcp-config.md) - 管理 Nginx 配置文件的各种操作
- [Nginx 服务管理](./mcp-nginx.md) - 控制和监控 Nginx 服务状态

## 接口

MCP 接口通过 `/mcp` 提供 Streamable HTTP 传输。将 OpenCode 等客户端的连接地址设置为此 URL，并通过 HTTP POST 发送 JSON-RPC 请求。初始化会直接返回 JSON-RPC 响应，无需先建立 SSE 连接。

对于使用会话的协议版本，初始化响应会包含 `Mcp-Session-Id` 响应头。后续请求都应携带此请求头，包括用于建立服务器事件流的 GET 请求。客户端也可以通过 `MCP-Protocol-Version` 请求头标识 Streamable HTTP GET 请求。

旧式 HTTP+SSE 客户端仍受支持：向 `/mcp` 发送不带上述两个请求头的 GET 请求会建立旧式 SSE 流，并通过事件告知客户端使用 `/mcp_message?sessionId=...` 作为消息端点。两种传输方式使用相同的认证机制和工具权限范围。

## 认证

在 **偏好设置 > 访问令牌** 中创建服务令牌，并仅授予客户端所需的最小 MCP 权限范围：

- `mcp:read` 允许读取资源和调用只读工具，已存储的密钥会被隐藏：`nginx_config_get` 拒绝读取私钥文件（`.key` 文件或任何包含 PEM 私钥块的文件），`nginx_config_history` 会将私钥版本的内容替换为 `__NGINX_UI_REDACTED__`。
- `mcp:write` 允许调用会修改状态的工具，并包含 MCP 读取权限，可读取私钥内容。

通过 `Authorization` 请求头发送令牌：

```http
Authorization: Bearer nui_pat_...
```

令牌仅显示一次。请将其保存在密钥管理器中，并尽可能设置过期时间。URL 查询参数中的凭据会被拒绝，以防止凭据通过日志和浏览器历史记录泄露。

### 资源（Resource）

资源是 MCP 提供的可读取信息，例如 Nginx 状态。

### 工具（Tool）

工具是 MCP 提供的可执行操作，例如重启 Nginx、修改配置文件等。

## 使用场景

MCP 主要用于以下场景：

1. AI 驱动的 Nginx 配置管理
2. 自动化运维工具集成
3. 第三方系统与 Nginx UI 的集成
4. 提供机器可读的 API 以便于自动化脚本使用
