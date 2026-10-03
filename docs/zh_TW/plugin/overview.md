---
outline: [2, 3]
---

# 外掛開發概覽

外掛可以在不修改 Nginx UI 本身的情況下擴充它的功能：為憑證簽發加入 DNS 服務商、加入通知通道和備份目的地、在網頁介面中加入頁面和面板、提供 nginx 設定範本等等。外掛可以從外掛目錄安裝，也可以上傳外掛套件或從伺服器上的目錄安裝，並且可以隨時啟用、停用和更新。

本指南的對象是外掛開發者。它說明了外掛必須提供什麼、可以依賴什麼，因此外掛可以用任何語言撰寫。

## 外掛的組成 {#what-a-plugin-is-made-of}

外掛是一個在根目錄包含 `plugin.json` [清單檔案](./manifest.md)的外掛套件。清單宣告以下三個部分中的一個或多個：

| 部分 | 內容 | 適用情境 |
| --- | --- | --- |
| `server` | 由 Nginx UI 啟動並與之通訊的可執行檔。 | 外掛需要在伺服器上執行程式碼：呼叫服務商 API、提供 HTTP 介面、回應能力呼叫。 |
| `webapp` | 載入到 Nginx UI 網頁介面中的 JavaScript 套件，或在框架中顯示的靜態 HTML 頁面。 | 外掛要在介面中加入頁面、面板、表單欄位或欄。 |
| `content` | nginx 設定範本和翻譯檔案。 | 外掛只提供資料。這類外掛完全沒有進程。 |

一個外掛可以同時包含這三個部分。例如官方 DNS-01 外掛既有與 DNS 服務商通訊的伺服器端進程，也有把欄位加入憑證表單的瀏覽器套件。

## 伺服器端進程如何運作 {#how-the-server-process-works}

Nginx UI 啟動可執行檔，並透過進程的標準輸入和標準輸出與它交換 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) 訊息，每行一個 JSON 物件。雙方都可以發送請求：

- Nginx UI 呼叫外掛來驅動它的[生命週期](./lifecycle.md)（啟動、設定、健康檢查、停止），並使用它的[能力](#capabilities)。
- 外掛透過[宿主 API](./host-api.md) 呼叫 Nginx UI：鍵值儲存、外掛設定、通知、排程工作、已儲存的認證。

外掛還可以透過 gRPC 提供同樣的呼叫以提升速度，而存取日誌串流必須使用 gRPC。[通訊協定](./protocol.md)一頁介紹了這兩種傳輸方式。

外掛進程以與 Nginx UI 相同的作業系統權限執行。[權限](./permissions.md)決定外掛可以使用 Nginx UI 的哪些功能，而不是它在機器上能做什麼，所以安裝外掛就意味著信任它的發佈者。[簽章](./signing.md)讓安裝外掛的人知道是誰發佈了它。

## 能力 {#capabilities}

能力是 Nginx UI 在自身功能之外交給外掛提供的功能。外掛在清單中宣告它實作的能力，每當用到該功能時，Nginx UI 就會呼叫它。

| 能力 | 外掛做什麼 | 頁面 |
| --- | --- | --- |
| `dns01` | 透過 DNS 服務商發佈 ACME DNS-01 驗證所需的 TXT 記錄。 | [DNS-01](./capabilities/dns01.md) |
| `http` | 在 Nginx UI 的身分驗證之後提供 HTTP API 或頁面。 | [HTTP 介面](./http.md) |
| `notify` | 透過聊天服務、推播閘道或告警系統傳送 Nginx UI 的通知。 | [通知通道](./capabilities/notify.md) |
| `probe` | 加入網站健康檢查的類型。 | [健康檢查](./capabilities/probe.md) |
| `mcp` | 為 AI 助理向 Nginx UI 的 MCP 伺服器加入工具。 | [MCP 工具](./capabilities/mcp.md) |
| `storage` | 把備份儲存到 Nginx UI 本身無法存取的地方。 | [儲存](./capabilities/storage.md) |
| `cert.deploy` | 把簽發的憑證推送到 CDN、負載平衡器和其他伺服器。 | [憑證部署](./capabilities/cert-deploy.md) |
| `security.blocklist` | 取得需要拒絕存取的位址清單。 | [封鎖清單](./capabilities/blocklist.md) |
| `upstream.discovery` | 把一個服務解析成 nginx upstream 的伺服器。 | [上游探索](./capabilities/discovery.md) |
| `log.sink` | 在 nginx 寫入存取日誌時即時接收日誌行。 | [存取日誌串流](./capabilities/log-sink.md) |

範本和翻譯檔案不需要能力，它們屬於[內容](./capabilities/content.md)。

## SDK {#sdks}

協定是透過管道傳輸的一般 JSON，因此任何語言都可以使用。SDK 替你處理協定細節：

| SDK | 語言 | 涵蓋範圍 |
| --- | --- | --- |
| [plugin-sdk-go](https://github.com/nginxui/plugin-sdk-go) | Go | 伺服器端進程：協定、生命週期、所有能力、gRPC、HTTP。 |
| [plugin-sdk-rust](https://github.com/nginxui/plugin-sdk-rust) | Rust | 伺服器端進程：協定、生命週期、所有能力、gRPC、HTTP。 |
| [@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) | TypeScript | 瀏覽器套件：執行環境型別、Vite 預設設定、靜態頁面輔助工具。 |

其他語言的外掛直接實作協定即可。[plugin-spec](https://github.com/nginxui/plugin-spec) 儲存庫存放了每個 SDK 所依據的機器可讀定義：所有訊息的 protobuf 定義、清單和外掛目錄的 JSON Schema、請求與回應範例，以及一個沒有任何相依套件的 Python 外掛範例。

## 下一步 {#where-to-go-next}

- [快速上手](./quick-start.md)：建立、建置並安裝第一個外掛。
- [開發與除錯](./development.md)：介紹發佈前檢查外掛的工具。
- [打包](./packaging.md)、[簽章與信任](./signing.md)和[外掛目錄](./catalog.md)：介紹如何發佈。
