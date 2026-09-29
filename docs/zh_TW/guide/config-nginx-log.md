# Nginx Log

Nginx UI 會列出從 Nginx 設定中發現的日誌檔案，以及 Nginx 預設的存取日誌和錯誤日誌。你可以分頁檢視日誌，也可以即時追蹤。這些功能無需任何設定。

## 日誌分析外掛

結構化搜尋、流量面板、訪客地圖和 IP 位置庫由官方外掛 **日誌分析**（`com.nginxui.log-analytics`）提供。它們以前以「進階索引」的形式內建在 Nginx UI 中。

- 在外掛頁面安裝並啟用該外掛。沒有外網的節點，可以在同一頁面上傳外掛安裝包。
- 啟用後會自動開始索引。停用外掛即停止索引，並釋放它佔用的全部記憶體。Nginx UI 本身不會載入這部分程式碼。
- 沒有安裝外掛時，日誌列表只顯示基礎欄位，日誌頁面只有原始檢視。
- 叢集中，每個需要分析日誌的節點都要安裝該外掛。

### 設定

原來 `[nginx_log]` 中的 `IncrementalIndexInterval`、`MaxConcurrentIndexTasks`、`IndexCustomMMDB` 和 `GeoMapPath` 不再由 Nginx UI 讀取，它們現在是外掛的設定，在外掛頁面的外掛設定中修改：

| 外掛設定 | 原來的設定 |
|----------|------------|
| 索引間隔（分鐘） | `IncrementalIndexInterval` |
| 同時索引的日誌數 | `MaxConcurrentIndexTasks` |
| 自訂 IP 位置庫 | `IndexCustomMMDB` |
| 地圖檔案資料夾 | `GeoMapPath` |

IP 位置庫（GeoLite2）也在外掛設定中下載。[template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb) 中用於產生自訂庫的指令碼仍然適用，把自訂庫的設定指向產生的檔案即可。

### 從進階索引升級

如果之前開啟了 `IndexingEnabled`，日誌頁面會提示「日誌分析已改為外掛」，從提示中前往安裝即可。外掛首次啟動時會接管既有的索引及其記錄、上述設定和已下載的 IP 位置庫，不會重複索引。之後 Nginx UI 會把 `IndexingEnabled` 關閉，並刪除舊的索引資料表。

`IndexingEnabled` 和 `IndexPath` 仍保留在 `app.ini` 中，也仍可用 `NGINX_UI_NGINX_LOG_INDEXING_ENABLED` 和 `NGINX_UI_NGINX_LOG_INDEX_PATH` 設定。它們只用於這次交接，網頁介面不能再修改。
