# MCP 模块

## 简介

MCP（Model Context Protocol）是 Nginx UI 提供的一个特殊接口，允许 AI 代理与 Nginx UI 交互。通过 MCP，AI 模型可以访问和管理 Nginx 配置文件、执行 Nginx 相关操作（如重启、重载）以及获取 Nginx 运行状态。

## 功能概览

MCP 模块主要分为两大部分功能：

- [配置文件管理](./mcp-config.md) - 管理 Nginx 配置文件的各种操作
- [Nginx 服务管理](./mcp-nginx.md) - 控制和监控 Nginx 服务状态

## 接口

MCP 接口通过 `/mcp` 路径提供 SSE 流式传输。

## 认证

MCP 接口通过 `node_secret` 查询参数进行认证。

例如：

```
http://localhost:9000/mcp?node_secret=<your_node_secret>
```

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
