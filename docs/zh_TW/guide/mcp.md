# MCP 模組

## 簡介

MCP（Model Context Protocol）是 Nginx UI 提供的一個特殊介面，允許 AI 代理與 Nginx UI 互動。通過 MCP，AI 模型可以訪問和管理 Nginx 配置文件、執行 Nginx 相關操作（如重啓、重載）以及獲取 Nginx 運行狀態。

## 功能概覽

MCP 模組主要分為兩大部分功能：

- [配置文件管理](./mcp-config.md) - 管理 Nginx 配置文件的各種操作
- [Nginx 服務管理](./mcp-nginx.md) - 控制和監控 Nginx 服務狀態

## 介面

MCP 介面通過 `/mcp` 路徑提供 SSE 流式傳輸。

## 認證

MCP 介面通過 `node_secret` 查詢參數進行認證。

例如：

```
http://localhost:9000/mcp?node_secret=<your_node_secret>
```

### 資源（Resource）

資源是 MCP 提供的可讀取信息，例如 Nginx 狀態。

### 工具（Tool）

工具是 MCP 提供的可執行操作，例如重啓 Nginx、修改配置文件等。

## 使用場景

MCP 主要用於以下場景：

1. AI 驅動的 Nginx 配置管理
2. 自動化運維工具集成
3. 第三方系統與 Nginx UI 的集成
4. 提供機器可讀的 API 以便於自動化腳本使用 