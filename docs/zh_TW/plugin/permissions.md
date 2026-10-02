---
outline: [2, 3]
---

# 權限與安全

## 外掛能做什麼 {#what-a-plugin-can-do}

::: danger 警告
外掛的伺服器端進程以與 Nginx UI 相同的作業系統使用者執行，沒有任何沙箱：它可以讀取該使用者能讀取的檔案，也可以像其他程式一樣建立網路連線。因此安裝外掛就意味著信任它的發佈者，[簽章與信任等級](./signing.md)就是幫助人們做這個判斷的。
:::

權限是另一回事。權限決定外掛可以使用哪些 **Nginx UI 功能**（它的鍵值儲存、已儲存的認證、通知、Nginx UI 的 REST API），以及 Nginx UI 會把哪些資料交給它（憑證、存取日誌）。權限也會告訴安裝外掛的人它將要做什麼。

每個外掛都有一個只有它自己可以寫入的資料目錄。Nginx UI 施加的資源限制只約束進程消耗多少資源，不約束它能存取什麼。

## 權限 {#permissions}

外掛在清單的 `permissions` 陣列中列出它需要的權限：

| 權限 | 允許的內容 |
| --- | --- |
| `kv` | 鍵值儲存：`host.kv.get`、`set`、`delete` 和 `list`。 |
| `network` | Nginx UI 不強制執行任何限制。它宣告外掛本身會建立對外網路連線，每個與服務商通訊的外掛都應請求它。`security.blocklist` 和 `upstream.discovery` 必須請求它。 |
| `cron` | 在執行期間用 `host.cron.register` 和 `unregister` 註冊排程呼叫。 |
| `notify` | 用 `host.notify` 在 Nginx UI 中發出通知。 |
| `metrics.read` | 用 `host.metrics.snapshot` 讀取 Nginx UI 的指標。 |
| `core_api` | 在瀏覽器套件中透過 `registry.coreHttp` 呼叫 Nginx UI 的 REST API。 |
| `mcp` | 把外掛的工具發佈給 MCP 用戶端。`mcp` 能力必須請求它。 |
| `cert.deploy` | 在 `deploy.push` 中接收憑證及其私鑰。`cert.deploy` 能力必須請求它。 |
| `log.read` | 在 `log.push` 中接收每一行 nginx 存取日誌。`log.sink` 能力必須請求它。 |
| `log.files` | 用 `host.logs.list` 列出 nginx 日誌檔案、接收 `log.paths_changed` 事件，並讀取這些檔案及其輪替副本。 |
| `nginx.snippet` | 用 `host.nginx.snippet.*` 維護 nginx 設定片段。每次變更時 Nginx UI 都會測試設定並重新載入 nginx。 |
| `nginx.config.read` | 用 `host.nginx.config.list` 和 `get` 讀取 nginx 設定檔。 |
| `sites.read` | 用 `host.sites.list` 列出站點。 |
| `certs.read` | 用 `host.certs.list` 列出憑證，不包含私鑰。 |
| `credentials.read:<kind>` | 用 `host.credentials.get` 讀取某一類已儲存的認證，例如 `credentials.read:dns`。 |

`host.log`、`host.settings.get`、`host.i18n.locale` 和 `host.activity.set` 不需要權限。

只請求外掛實際用到的權限。檢查工具會對沒有能力用到的權限發出警告，也會對存取外部服務卻沒有 `network` 的能力發出警告。

### 已授予與已請求的權限 {#granted-and-requested-permissions}

清單列出外掛請求的權限，安裝外掛的人核准這份清單，只有已核准的權限才算數：呼叫需要外掛未持有的權限時，會以[權限拒絕錯誤](./protocol.md#error-codes)失敗，並且不執行任何操作。

當更新請求了尚未核准的權限時，Nginx UI 不會靜默授予。在有人核准新的權限清單之前，更新後的外掛保持停止，它的能力也無法使用。只減少權限的更新不需要核准；被移除的權限若在之後的版本中再次請求，需要重新核准。因此執行中的外掛一律持有清單請求的全部權限。交握會告訴進程它持有哪些權限，參見[生命週期](./lifecycle.md#handshake)。

### 敏感權限 {#sensitive-permissions}

有些權限會把需要格外小心的資料交給外掛，Nginx UI 在請求核准時會明確說明：

- **`mcp`** 允許 Nginx UI 授權的每個 MCP 用戶端執行外掛的工具。請把工具參數視為不可信的輸入。
- **`cert.deploy`** 會把使用者繫結到外掛目標的每張憑證的私鑰交給外掛。請把私鑰當作認證對待，呼叫結束後不要保留。
- **`log.read`** 和 **`log.files`** 會把 nginx 日誌交給外掛：用戶端位址、每個請求的 URL 及其查詢字串、參照頁面和使用者代理。在許多司法管轄區，這些都屬於個人資料。請把每個欄位視為不可信的輸入，絕不要寫入自己的日誌。
- **`nginx.snippet`** 讓外掛在使用者引入其片段的位置改變 nginx 提供的服務。Nginx UI 只在整體設定保持有效時才套用片段，但引入片段的人把這部分設定交給了外掛。請在要求使用者引入片段之前，先說明它的作用。
- **`security.blocklist`** 和 **`upstream.discovery`** 讓外掛決定 nginx 提供什麼服務。Nginx UI 會解析外掛回傳的每個位址和連接埠，絕不會把其他文字複製到 nginx 設定中，但依賴封鎖清單的人仍然需要信任其外掛不會把他們自己擋在門外。

## 宣告連線的主機 {#declaring-network-hosts}

`network_hosts` 列出外掛打算連線的主機：

```json [plugin.json]
"network_hosts": ["api.mydns.example"]
```

Nginx UI 會把這份清單顯示給安裝外掛的人。它僅供參考，不會阻止連線到其他主機。

## 說明權限用途 {#explaining-permissions}

`permission_reasons` 用一句話說明外掛為什麼需要某項權限，讓使用者可以對照外掛的實際功能來判斷：

```json [plugin.json]
"permissions": ["log.files", "network"],
"permission_reasons": {
  "log.files": "To index the access and error logs of your sites for search and the dashboard.",
  "network": "To download the IP location database when you choose to."
},
"i18n": {
  "zh_TW": {
    "permission_reasons": { "network": "在你選擇下載時取得 IP 位置資料庫。" }
  }
}
```

凡是請使用者授予權限的地方，包括市集、安裝對話框和新權限的審核，Nginx UI 都會在權限下方以作者說明的形式顯示這段原因。沒有寫原因或原因為空的權限只顯示 Nginx UI 自己的說明。某個語言的翻譯為空時，顯示英文原因。

每個鍵都必須是 `permissions` 中的一項，原因最多 300 個字元。鍵不在其中或原因過長時，[`nginx-ui plugin lint`](./rules.md#manifest-permission-reasons) 會回報，Nginx UI 也會拒絕安裝這樣的 manifest。請說明用途，而不是解釋權限本身：每項權限允許做什麼，Nginx UI 已經有說明。

## 處理認證 {#handling-credentials}

::: danger 警告
認證指使用者以機密形式輸入的任何值（DNS 服務商的認證欄位、`secret` 型別的設定、通道、目標或後端中 `secret` 型別的設定欄位）、`host.credentials.get` 回傳的任何內容，以及 `deploy.push` 攜帶的私鑰。認證絕不能出現在：

- 回傳給 Nginx UI 的錯誤訊息中。請改用設定無效錯誤及其 `field` 詳細資料指出欄位；
- `host.log` 呼叫中；
- 外掛的標準錯誤中。
:::

如果有助於除錯，可以記錄一個值已收到的遮罩形式，例如它的長度或最後四個字元，而不是什麼都不記錄。Go SDK 的 `sdk.Redact` 和 Rust SDK 的 `redact` 就是做這個的。

::: warning 注意
請把能力 `config` 中的每個值都當作機密，而不只是標記為 `secret` 的欄位：Webhook 網址中常常包含權杖。
:::

透過環境變數把認證傳給函式庫的外掛（很多 DNS 函式庫從環境變數讀取認證）必須在呼叫結束後立即還原原本的值，使使用不同認證的兩次呼叫互不可見，並且絕不能把它們寫入任何持久化位置。

Nginx UI 也遵守同樣的要求：它加密儲存機密，絕不把已儲存的機密傳回瀏覽器，也絕不在日誌中寫入機密的值。
