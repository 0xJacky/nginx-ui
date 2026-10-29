# MCP 模組

## 簡介

MCP（Model Context Protocol）是 Nginx UI 提供的一個特殊介面，允許 AI 代理與 Nginx UI 互動。通過 MCP，AI 模型可以訪問和管理 Nginx 配置文件、執行 Nginx 相關操作（如重啓、重載）以及獲取 Nginx 運行狀態。

## 功能概覽

MCP 模組主要分為兩大部分功能：

- [配置文件管理](./mcp-config.md) - 管理 Nginx 配置文件的各種操作
- [Nginx 服務管理](./mcp-nginx.md) - 控制和監控 Nginx 服務狀態

## 介面

MCP 介面透過 `/mcp` 提供 Streamable HTTP 傳輸。將 OpenCode 等用戶端的連線位址設為此 URL，並透過 HTTP POST 傳送 JSON-RPC 請求。初始化會直接傳回 JSON-RPC 回應，無需先建立 SSE 連線。

對於使用工作階段的協定版本，初始化回應會包含 `Mcp-Session-Id` 回應標頭。後續請求都應攜帶此請求標頭，包括用於建立伺服器事件串流的 GET 請求。用戶端也可以透過 `MCP-Protocol-Version` 請求標頭識別 Streamable HTTP GET 請求。

舊式 HTTP+SSE 用戶端仍受支援：向 `/mcp` 傳送不帶上述兩個請求標頭的 GET 請求會建立舊式 SSE 串流，並透過事件告知用戶端使用 `/mcp_message?sessionId=...` 作為訊息端點。兩種傳輸方式使用相同的認證機制和工具權限範圍。

## 認證

在 **偏好設定 > 存取權杖** 中建立服務權杖，並僅授予用戶端所需的最小 MCP 權限範圍：

- `mcp:read` 允許讀取資源和呼叫唯讀工具，已儲存的密鑰會被隱藏：`nginx_config_get` 拒絕讀取私鑰檔案（`.key` 檔案或任何包含 PEM 私鑰區塊的檔案），`nginx_config_history` 會將私鑰版本的內容替換為 `__NGINX_UI_REDACTED__`。
- `mcp:write` 允許呼叫會修改狀態的工具，並包含 MCP 讀取權限，可讀取私鑰內容。

透過 `Authorization` 請求標頭傳送權杖：

```http
Authorization: Bearer nui_pat_...
```

權杖僅顯示一次。請將其儲存在金鑰管理器中，並盡可能設定到期時間。URL 查詢參數中的憑證會被拒絕，以防止憑證透過日誌和瀏覽器歷史記錄洩漏。

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