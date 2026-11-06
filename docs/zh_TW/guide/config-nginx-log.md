# Nginx Log

Nginx UI 會列出從 Nginx 設定中找到的日誌檔案，以及 Nginx 預設的存取日誌和錯誤日誌。你可以分頁檢視日誌，也可以即時追蹤。這些功能不需要任何設定。

## 日誌分析

日誌分析內建在 Nginx UI 中。開啟索引後，Nginx UI 會為列出的存取日誌和錯誤日誌建立索引，並提供：

- 結構化搜尋，支援篩選條件、時間範圍和分頁結果，存取日誌和錯誤日誌皆可使用；
- 流量面板，包括請求數、訪客數、狀態碼、熱門頁面、瀏覽器、作業系統和裝置；
- 依國家顯示的訪客地圖；安裝 GeoLite2 City 資料庫後，還可以細分到省份和城市。

在日誌列表中點選 **啟用高級索引**，或設定下方的 [IndexingEnabled](#indexingenabled) 即可開啟索引。Nginx UI 會先為既有日誌建立索引，之後只讀取新附加的內容。輪替後的檔案，包括被 logrotate 重新命名、被 `copytruncate` 複製以及被壓縮為 `.gz` 的檔案，會依內容辨識，不會重複索引。

索引是一個欄式儲存，存放在 [IndexPath](#indexpath) 下的 `logstore` 目錄中，體積通常遠小於原始日誌。

::: tip 從 v2 升級
v2 使用 Bleve 索引。v3 首次啟動時會刪除舊的索引目錄，並把所有日誌重新索引到新的儲存中。不需要任何操作，但日誌較大時重建需要一些時間。
:::

### 搜尋語法

搜尋框中的關鍵字和 `欄位:值` 篩選條件之間是「且」的關係。在條件前加上 `-` 表示排除，用雙引號可以把一個片語當作整體。一般關鍵字以不區分大小寫的子字串比對原始日誌行。

| 篩選條件 | 範例 |
|----------|------|
| 狀態碼、狀態碼類別或範圍 | `status:404`、`status:5xx`、`status:400-499`、`status:>=500` |
| 請求方法 | `method:POST` |
| 用戶端位址或網段 | `ip:10.0.0.0/8`、`ip:2001:db8::/32` |
| 路徑、User Agent、Referer（子字串） | `path:/api/`、`ua:curl`、`referer:google` |
| 瀏覽器、作業系統、裝置、省份、城市（子字串） | `browser:firefox`、`os:android`、`city:taipei` |
| 國家代碼 | `country:US` |
| 傳送位元組數 | `bytes:>1000` |
| 請求耗時（秒） | `rt:>0.5`、`rt:0.1..0.5` |
| 錯誤日誌層級 | `level:error`、`-level:warn` |

看起來像篩選條件但無法辨識的內容會當作一般文字搜尋，日誌頁面會對此顯示提示。

### IP 地理位置

國家資訊來自 Nginx UI 內建的 GeoLite2 Country 資料庫，因此不需下載任何檔案即可顯示世界地圖。省份和城市需要 GeoLite2 City 資料庫，可以在 **偏好設定 > GeoLite** 中下載，檔案會儲存為 `app.ini` 同一目錄下的 `GeoLite2-City.mmdb`。也可以使用自訂資料庫，請參閱 [IndexCustomMMDB](#indexcustommmdb)。

地理位置在日誌行建立索引時寫入。安裝或更換資料庫後，需要重建對應日誌的索引，已索引的日誌行才會使用新的資料。

## IndexingEnabled

- 類型：`boolean`
- 預設值：`false`
- 環境變數：`NGINX_UI_NGINX_LOG_INDEXING_ENABLED`
- 版本：`>= v2.2.0`

開啟日誌索引及上述分析功能。關閉時，日誌列表只顯示偵測到的日誌檔案及其基本資訊，日誌頁面只有原始檢視。即時追蹤日誌不受此項影響。

## IndexPath

- 類型：`string`
- 預設值：`app.ini` 所在目錄下的 `log-index`
- 環境變數：`NGINX_UI_NGINX_LOG_INDEX_PATH`
- 版本：`>= v2.2.0`

索引所在目錄，請使用絕對路徑。如果無法確定 `app.ini` 所在目錄，則使用工作目錄下的 `./log-index`。

## IncrementalIndexInterval

- 類型：`int`（分鐘）
- 預設值：`15`
- 環境變數：`NGINX_UI_NGINX_LOG_INCREMENTAL_INDEX_INTERVAL`
- 版本：`>= v2.2.0`

Nginx UI 檢查日誌新增內容的頻率。數值越小，分析資料越接近即時，背景 CPU 使用率也越高。設定為 `0` 或負數時使用預設的 15 分鐘。

## MaxConcurrentIndexTasks

- 類型：`int`
- 預設值：`0`（自動計算）
- 環境變數：`NGINX_UI_NGINX_LOG_MAX_CONCURRENT_INDEX_TASKS`
- 版本：`>= v2.5.7`

限制同時建立索引的日誌群組數量，包括首次索引、重新啟動後的復原工作以及增量索引工作。每個正在索引的日誌群組都會在記憶體中持有解析批次，因此這個值是控制索引期間記憶體峰值的主要方式。

設定為 `0` 或負數**並不**代表不限制。此時上限會依程序可用的 CPU 數量計算（若有設定容器 CPU 配額，則以配額為準）：取其一半，最少為 `1`，最多為 `2`。設定為正數可覆寫此值，例如在索引與 Nginx 爭用記憶體的小規格主機上設為 `1`。

## IndexCustomMMDB

- 類型：`string`
- 預設值：空（使用 GeoLite2 City 資料庫）
- 環境變數：`NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB`

指定自訂 MaxMind DB（`.mmdb`）檔案的路徑，用於 IP 地理位置解析。自訂記錄可以提供國家、省份、城市及四個業務標籤（`c1` 至 `c4`），例如分公司、工廠、部門和網路類型。

絕對路徑直接使用設定值。相對路徑以目前使用的 `app.ini` 所在目錄為基準解析，而不是程序的工作目錄。例如，將 `enterprise.mmdb` 放在 `app.ini` 同一目錄下，並設定：

```ini
[nginx_log]
IndexingEnabled = true
IndexCustomMMDB = enterprise.mmdb
```

也可以透過環境變數指定 Nginx UI 程序可存取的路徑：

```bash
NGINX_UI_NGINX_LOG_INDEX_CUSTOM_MMDB=/etc/nginx-ui/enterprise.mmdb
```

使用 Docker 部署時，需要將資料庫掛載到容器內，並填寫容器內的路徑。執行 Nginx UI 的使用者必須具有該檔案的讀取權限。

::: warning 資料庫選擇規則
如果 `app.ini` 同一目錄下存在 `GeoLite2-City.mmdb`，它的優先順序高於 `IndexCustomMMDB`。要使用自訂資料庫，請先將標準資料庫移到備份位置。兩個資料庫不會合併；未符合自訂 IP 範圍的位址也不會改用標準城市資料庫查詢。

設定路徑不會自動下載或產生檔案。
:::

設定了 `IndexCustomMMDB` 時，**偏好設定 > GeoLite** 會顯示所設定的自訂資料庫，並隱藏 GeoLite2 資料庫的下載操作。

### 產生自訂資料庫

儲存庫的 [template/custom-mmdb](https://github.com/0xJacky/nginx-ui/tree/dev/template/custom-mmdb) 目錄提供了產生指令碼和範例資料。

1. 準備 Python 3 環境及產生指令碼所需的相依套件：`mmdb_writer` 和 `netaddr`。
2. 編輯 `region_codes.json`，定義國家、省份和城市的層級關係。附帶的檔案僅作為起始範本；引用尚未列出的地區前，請先補上對應資料。
3. 編輯 `ip_inventory.json`，將單一 IPv4 位址或 CIDR 網段對應到上述地區層級及業務標籤。四個標籤鍵都需要保留，未使用的標籤填入空字串。

例如，蘇州某網段的清單項目可以寫成：

```json
{
  "10.10.0.0/16": {
    "country": "CN",
    "province": "320000",
    "city": "320500",
    "c1": "蘇州分公司",
    "c2": "工廠 A",
    "c3": "生產 IT 部門",
    "c4": "有線網路"
  }
}
```

在儲存庫根目錄執行產生指令碼：

```bash
python3 template/custom-mmdb/Build_Custom_mmdb.py
```

指令碼會驗證網路位址和地區引用，然後在 `template/custom-mmdb` 中產生 `enterprise.mmdb` 及清單匯出檔 `enterprise_data.json`。附帶的產生指令碼建立的是 IPv4 資料庫，單一 IPv4 位址會轉換為 `/32` 網段。

將 `enterprise.mmdb` 複製到設定指定的位置，並在修改設定或替換資料庫後重新啟動 Nginx UI。然後重建需要套用新地理資訊和業務標籤的日誌索引。

## GeoMapPath

- 類型：`string`
- 預設值：空（即 `app.ini` 所在目錄下的 `maps`）
- 環境變數：`NGINX_UI_NGINX_LOG_GEO_MAP_PATH`

地圖使用的中國及省級邊界 GeoJSON 檔案所在目錄，檔名需遵循 `100000_full.json`、`<省級adcode>_full.json` 的命名規則。預設只提供世界地圖，邊界檔案需要自行下載並放入該目錄。

- 設定為絕對路徑時，直接使用該路徑。
- 設定為相對路徑時，以 `app.ini` 所在目錄為基準解析。
- 沒有本機檔案時，儀表板在中文語系下仍可顯示中國地圖，邊界資料會從 CDN 載入。

範例：

```ini
[nginx_log]
GeoMapPath = /etc/nginx-ui/maps
```
