---
outline: [2, 3]
---

# 生命週期

本頁跟隨外掛進程從啟動到結束的整個過程。它適用於帶有 `server` 區塊的外掛，沒有 `server` 區塊的外掛沒有進程。

Nginx UI 用六個方法驅動生命週期：

| 方法 | 類型 | 用途 |
| --- | --- | --- |
| `plugin.initialize` | 請求 | 交握：版本、能力、設定、權限。 |
| `plugin.initialized` | 通知 | 交握成功，外掛可以呼叫宿主 API。 |
| `plugin.configure` | 請求 | 設定已變更。 |
| `plugin.ping` | 請求 | 健康檢查。 |
| `plugin.shutdown` | 請求 | 完成進行中的工作，準備結束。 |
| `plugin.exit` | 通知 | 立即結束。 |

::: tip 提示
SDK 實作了所有這些方法，你只需提供設定變更和關閉時要做的事。
:::

## 交握 {#handshake}

啟動進程後，Nginx UI 首先傳送 `plugin.initialize`：

```json
{
  "jsonrpc": "2.0", "id": 1, "method": "plugin.initialize",
  "params": {
    "host": { "version": "2.7.0", "os": "linux", "arch": "amd64", "locale": "en" },
    "settings": {},
    "permissions": ["network"]
  }
}
```

- `settings` 是外掛已儲存的設定，首次安裝時為 `{}`。
- `permissions` 是外掛持有的權限。它一律與清單一致，因為 Nginx UI 不會啟動權限尚待核准的外掛。

外掛以它實作的內容回覆：

```json
{ "jsonrpc": "2.0", "id": 1, "result": { "api_version": 1, "capabilities": ["dns01"] } }
```

| 欄位 | 意義 |
| --- | --- |
| `api_version` | 進程的協定世代，必須等於 Nginx UI 實作的世代 `1`。 |
| `capabilities` | 執行中的進程實作的能力。作為集合必須與清單的 `capabilities` 完全一致。 |
| `transports` | 外掛提供的傳輸方式，例如 `["stdio", "grpc"]`。標準輸入輸出總是可用；列出 `grpc` 即啟用 [gRPC 傳輸](./protocol.md#grpc-transport)。 |
| `rpc_socket`、`rpc_pipe`、`rpc_port`、`rpc_token` | gRPC 傳輸的監聽位置，參見[通訊協定](./protocol.md#grpc-transport)。 |
| `http_pipe`、`http_port` | Windows 上的外掛提供 HTTP 介面的位置，參見 [HTTP 介面](./http.md#where-to-listen)。 |

`api_version` 為其他值，或者能力清單與清單檔案有任何差異，交握都會失敗。Nginx UI 最多等待 **10 秒**，因此請把啟動時的工作（載入大型目錄、預熱快取）控制在這個時間內，或者放到交握之後進行。

交握成功後，Nginx UI 傳送 `plugin.initialized`。從這時起外掛才可以呼叫[宿主 API](./host-api.md)，更早的呼叫可能會被拒絕。

## 設定 {#settings}

外掛執行期間，每當使用者儲存外掛的設定，Nginx UI 就會傳送新的設定：

```json
{ "jsonrpc": "2.0", "id": 2, "method": "plugin.configure", "params": { "settings": { "timeout": 90 } } }
```

外掛在後續呼叫使用新值之後回覆 `{}`。交握時的設定不會作為 `plugin.configure` 呼叫重複傳送，不過 Nginx UI 可能在交握後立即傳送一次。`host.settings.get` 可以隨時回傳目前的設定。

## 健康檢查 {#health-checks}

進程執行期間，Nginx UI 每 **15 秒**傳送一次 `plugin.ping`，並要求在 **5 秒**內收到 `{}`。連續 **3** 次沒有回應，它就會終止進程，並以當機處理。

::: warning 注意
外掛即使在忙於其他呼叫時也必須回應 ping。請獨立於耗時工作處理 ping，至少也要優先處理它。SDK 會替你做到這一點。
:::

## 停止 {#stopping}

外掛被停用、解除安裝、更新、（`on_demand` 外掛）閒置足夠久，或 Nginx UI 關閉時，Nginx UI 會停止外掛：

1. 傳送 `plugin.shutdown`，最多等待 **5 秒**的回覆。外掛應利用這一步完成進行中的呼叫（包括未結束的 gRPC 呼叫和日誌串流），並拒絕新的呼叫，然後回覆 `{}`。
2. 傳送 `plugin.exit`，無論是否收到了回覆。
3. 最多等待 **5 秒**讓進程結束。
4. 終止進程。

收到 `plugin.exit` 後，外掛應盡快結束，不再等待輸入，並順便關閉 gRPC 和 HTTP 監聽，以狀態碼 `0` 結束。任何其他狀態碼，或者沒有經過這些訊息就結束，都視為當機。

## 當機與重新啟動 {#crashes-and-restarts}

自行結束的 `resident` 進程視為當機。Nginx UI 會在 1、2、4、8 秒後以及之後每 16 秒重新啟動它。5 分鐘內當機 3 次後，它就不再嘗試，並把外掛標記為失敗，直到有人手動重新啟動。

`on_demand` 進程在首次需要時啟動，在 `idle_timeout_seconds` 內沒有呼叫後停止，下一次呼叫會再次啟動它。沒有人等待它時發生的當機不計入限制。

::: warning 注意
進程也可能在任何時刻不經停止流程就被終止，例如超出記憶體限制時。請不要依賴 `plugin.shutdown` 來保持資料目錄中檔案的一致性：先寫入暫存檔再重新命名，或者使用能在當機後復原的資料庫。
:::

## 環境變數 {#environment}

Nginx UI 為進程設定以下變數：

| 變數 | 意義 |
| --- | --- |
| `NGINX_UI_PLUGIN_ID` | 外掛 ID。 |
| `NGINX_UI_PLUGIN_API_VERSION` | Nginx UI 實作的協定世代，例如 `1`。 |
| `NGINX_UI_PLUGIN_DATA_DIR` | 只有本外掛可以寫入的目錄的絕對路徑。所有狀態都儲存在這裡。 |
| `NGINX_UI_VERSION` | Nginx UI 的版本。 |
| `NGINX_UI_DEMO` | Nginx UI 作為公開展示執行時為 `1`，外掛可以用預留位置內容取代展示環境無法提供的資料。否則不設定。 |
| `NGINX_UI_PLUGIN_HTTP_SECRET` | 提供 HTTP 介面的外掛所用的密鑰。參見 [HTTP 介面](./http.md)。 |

### 對外代理 {#outbound-proxy}

當 Nginx UI 設定了對外 HTTP 代理時，持有 `network` 權限的外掛會透過 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY`（及其小寫形式）收到代理設定，大多數語言的標準 HTTP 用戶端不需額外程式碼就會使用它。`NO_PROXY` 總是包含回送位址。這些變數在進程啟動時讀取，因此代理設定變更後，外掛在下次啟動時才會生效。沒有 `network` 權限的外掛永遠不會收到它們。

## 資源限制 {#resource-limits}

在使用 cgroup v2 的 Linux 上，Nginx UI 可以限制外掛進程的記憶體和 CPU 時間。維運人員在 **偏好設定 > 外掛** 中設定限制，外掛的 [`server.resources`](./manifest.md#resources) 提示值可以降低限制，但不能提高限制。每個外掛執行在自己的群組 `<cgroup 根目錄>/nginx-ui/plugins/<外掛 ID>` 中，並停用置換空間。

超出記憶體限制的進程會被立即終止並以當機處理，因此持續超出限制的外掛最終會被標記為失敗。在無法強制執行限制的環境中（其他作業系統、權限不足、未委派 cgroup 的容器），外掛在沒有限制的情況下執行，其詳細資料會說明限制未生效。

## 衝突的外掛 {#conflicting-plugins}

兩個[衝突](./manifest.md#dependencies-and-conflicts)的外掛永遠不會同時執行：

- 衝突的外掛已啟用時，啟用外掛會失敗，錯誤訊息會指出另一個外掛，除非使用者選擇取代它（先停用它）。
- 一步完成安裝和啟用的外掛套件，在衝突的外掛已啟用時保持停用，除非選擇了取代。
- 啟動時，與保持啟用的外掛衝突的外掛會被停用，並記錄一筆指出另一個外掛的錯誤。
