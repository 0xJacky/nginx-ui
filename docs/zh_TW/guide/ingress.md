# Ingress API

**Ingress** 是由外部控制器宣告、並由 Nginx UI 負責維持的反向代理入口。控制器傳送期望狀態；Nginx UI 負責產生 nginx 設定、簽發憑證、測試並重新載入 nginx，然後回報結果。以這種方式產生的網站會被標記為受管理，不應手動編輯：下一次調和會將其覆寫。

典型用法：控制平面透過通道（例如綁定在 `127.0.0.1` 上的 frp 遠端連接埠）將服務連到這台主機，並以 443 連接埠上的公開主機名稱對外提供服務。

## 認證

所有端點都位於 `/api` 之下，並接受一般的認證方式：

- 已簽章的節點請求（`Signature` / `Signature-Input`）或舊版的 `X-Node-Secret` 標頭 — 建議控制器使用；
- 具有 `api:read` / `api:write` 範圍的服務權杖。

寫入端點需要安全的工作階段；節點請求與 `api:write` 權杖皆符合此要求。`owner` 欄位用於防止兩個控制器互相覆寫，並非授權邊界。

## 資源

```json
{
  "id": 7,
  "name": "tm-prod-insure-ocr",
  "owner": "telemetry-prod",
  "external_id": "mac-service:42",
  "labels": { "service": "insure-ocr" },
  "namespace_id": 0,
  "generation": 3,
  "site_path": "ingress-tm-prod-insure-ocr",
  "spec": {
    "hosts": ["ocr.example.com"],
    "backends": [
      {
        "name": "app",
        "lb": "round_robin",
        "keepalive": 16,
        "servers": [
          { "address": "127.0.0.1:24001", "weight": 1, "max_fails": 2, "fail_timeout": "10s", "down": false, "backup": false },
          { "address": "127.0.0.1:24002", "down": true, "backup": false }
        ]
      }
    ],
    "routes": [
      {
        "path_prefix": "/",
        "backend": "app",
        "websocket": true,
        "read_timeout": "300s",
        "send_timeout": "300s",
        "client_max_body_size": "64m",
        "allow_cidrs": []
      }
    ],
    "tls": {
      "mode": "acme",
      "cert_id": 0,
      "key_type": "2048",
      "challenge": "http01",
      "dns_credential_id": 0,
      "acme_user_id": 0,
      "redirect_https": true,
      "hsts": false
    },
    "policy": {
      "allow_cidrs": ["203.0.113.0/24"],
      "rate_limit": { "rps": 20, "burst": 40 },
      "client_max_body_size": "16m",
      "extra_headers": [{ "name": "X-Request-Source", "value": "ingress" }]
    }
  },
  "status": {
    "observed_generation": 3,
    "phase": "Ready",
    "conditions": [
      { "type": "ConfigApplied", "status": true, "reason": "Applied", "message": "", "last_transition_time": "2026-10-15T08:00:00Z" },
      { "type": "CertReady", "status": true, "reason": "Issued", "message": "", "last_transition_time": "2026-10-15T08:01:10Z" },
      { "type": "BackendsHealthy", "status": true, "reason": "AllHealthy", "message": "", "last_transition_time": "2026-10-15T08:01:30Z" }
    ],
    "config_hash": "9f1c…",
    "cert_id": 31,
    "cert_not_after": "2027-01-13T08:01:00Z",
    "last_applied_at": "2026-10-15T08:01:12Z",
    "servers": [
      { "backend": "app", "address": "127.0.0.1:24001", "down": false, "healthy": true },
      { "backend": "app", "address": "127.0.0.1:24002", "down": true, "healthy": false }
    ]
  },
  "created_at": "2026-10-15T07:59:58Z",
  "updated_at": "2026-10-15T08:01:30Z"
}
```

### Spec 規則

| 欄位 | 規則 |
|---|---|
| `name` | 1–40 個字元，`[a-z0-9-]`，以字母或數字開頭與結尾；在所有擁有者之間不得重複 |
| `owner` | 1–64 個字元，`[A-Za-z0-9._-]` |
| `hosts` | 至少一個；DNS 名稱（允許 IDN，依原樣儲存並以 punycode 呈現）；`*.` 萬用字元僅能搭配 `tls.mode` 為 `cert` 或 `none`，或 `dns01` 驗證方式；一個主機只能屬於一個 Ingress |
| `backends[].name` | 1–16 個字元，`[a-z0-9-]`；在該 Ingress 內不得重複 |
| `backends[].lb` | `round_robin`（預設）、`least_conn`、`ip_hash` |
| `backends[].servers[].address` | `host:port`，其中 IP 主機必須位於 `[ingress] AllowedBackends` 之內（預設為 `127.0.0.1/32`、`::1/128`）；每個 backend 至少要有一個 server |
| `routes[].path_prefix` | 以 `/` 開頭，不得包含空格、`;`、`{`、`}`、引號或 `#`；在該 Ingress 內不得重複；最長前綴優先 |
| `routes[].backend` | 必須是該 Ingress 中某個 backend 的名稱 |
| timeouts | nginx 時間值（`60s`、`5m`） |
| sizes | nginx 大小值（`0`、`512k`、`64m`、`1g`） |
| `allow_cidrs` | IPv4/IPv6 CIDR 或位址；空值表示所有人皆可；路由層級的清單會取代該路由的 policy 清單 |
| `tls.mode` | `none`、`acme`（簽發並續期）、`cert`（使用 `cert_id`） |
| `tls.challenge` | `http01`（預設）或 `dns01`（需要 `dns_credential_id`） |
| `tls.key_type` | 與憑證簽發相同（`2048`、`3072`、`4096`、`P256`、`P384`）；預設為 `2048` |
| `policy.rate_limit` | 依用戶端位址計算；`rps` 為 1–10000，`burst` 為 0–100000 |
| `policy.extra_headers` | 標頭名稱為 `[A-Za-z0-9-]`，值不得包含 `;`、`{`、`}`、引號與換行 |

## 產生的檔案

| 檔案 | 內容 |
|---|---|
| `sites-available/ingress-<name>` 與 `sites-enabled` 連結 | server 區塊 |
| `conf.d/upstream-ig-<name>-<backend>.conf` | 每個 backend 一個受管理的上游，並附帶共用記憶體區域 |

網站資料列的 `managed_by` 會被設為 `ingress:<name>`，且網站 API 會回傳該值。一般的網站 API 會拒絕對這類網站進行儲存、重新命名、啟用、停用、刪除或進入維護模式（單筆與批次端點皆然），並回傳 `ErrSiteManagedByIngress`，除非請求強制執行：`POST /sites/:name` 的 JSON 主體中帶 `"force": true`，其他端點則帶 `?force=1`。被強制執行的變更會在下一次調和時被覆寫。這些上游會與其他受管理的上游一樣，顯示在上游頁面與可用性檢查器中。

渲染出的 server 區塊大致如下：

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name ocr.example.com;
    # ACME HTTP-01 challenge, proxied to Nginx UI
    location ^~ /.well-known/acme-challenge { ... }
    location / { return 301 https://$host$request_uri; }   # tls.redirect_https and cert ready
}
server {                                                    # only when the certificate is ready
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name ocr.example.com;
    ssl_certificate     /etc/nginx/ssl/ocr.example.com_2048/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/ocr.example.com_2048/private.key;
    location / {
        allow 203.0.113.0/24; deny all;
        limit_req zone=ig_tm-prod-insure-ocr burst=40 nodelay;
        client_max_body_size 64m;
        proxy_pass http://ig-tm-prod-insure-ocr-app;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;             # websocket
        proxy_set_header Connection $connection_upgrade;    # websocket
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

## 端點

| 方法 | 路徑 | 主體 | 回應 |
|---|---|---|---|
| GET | `/ingresses?owner=<owner>` | — | `{ "data": [Ingress] }` |
| GET | `/ingresses/:name` | — | `Ingress` |
| PUT | `/ingresses/:name?wait=<duration>&dry_run=1` | `UpsertRequest` | `Ingress`，試運行時為 `ApplyResult` |
| DELETE | `/ingresses/:name?owner=<owner>&force=1&purge_cert=1` | — | `{ "message": "ok" }` |
| PUT | `/ingresses?owner=<owner>&prune=1&wait=<duration>` | `SyncRequest` | `{ "data": [Ingress] }`（同步後該擁有者的 Ingress） |
| PATCH | `/ingresses/:name/backends/:backend/servers?wait=<duration>` | `ServerPatch` | `Ingress` |
| POST | `/ingresses/:name/reconcile` | — | `Ingress` |

```jsonc
// UpsertRequest
{ "owner": "telemetry-prod", "external_id": "mac-service:42", "labels": {}, "namespace_id": 0, "force": false, "spec": { /* IngressSpec */ } }

// SyncRequest: every ingress the owner wants; names not listed are deleted when prune=1
{ "items": [ { "name": "tm-prod-insure-ocr", "external_id": "…", "labels": {}, "namespace_id": 0, "spec": { /* IngressSpec */ } } ] }

// ServerPatch
{ "owner": "telemetry-prod", "address": "127.0.0.1:24002", "down": true }

// ApplyResult (dry run)
{ "name": "…", "changed": true, "rendered": { "site_name": "…", "site_content": "…", "upstreams": { "ig-…-app": "…" }, "hash": "…" }, "test_output": "", "error": "" }
```

語意說明：

- **PUT 是冪等的。** 只有在 spec 變更時，版本（generation）才會增加；無論如何，該 Ingress 都會被排入套用佇列，因此重複送出 PUT 即可重試失敗的套用。
- **擁有者。** 建立時會設定擁有者。以不同的擁有者寫入既有 Ingress 會失敗並回傳 `ErrOwnerMismatch`（409），除非設定 `force`，此時會轉移擁有權。帶有 `prune=1` 的同步只會刪除指定擁有者的 Ingress。
- **套用是非同步的。** 寫入在 spec 儲存完成後即回傳。約一秒內抵達的變更會一併套用，只執行一次 `nginx -t` 與一次重新載入。若指定 `wait`，請求會阻塞到 `status.observed_generation` 達到新版本，或等待逾時（`ErrWaitTimeout`，408；變更仍保留在佇列中）。未指定時請輪詢 `GET /ingresses/:name`。
- **套用失敗是原子性的。** 該次套用寫入的所有檔案都會還原，nginx 繼續執行先前的設定，而失敗的 Ingress 會得到 `ConfigApplied=false`，測試輸出則放在訊息中。
- **摘除。** `PATCH …/servers` 搭配 `down: true` 會在該 server 上設定 `down`；重新載入後 nginx 不再向它傳送新請求，進行中的請求則會完成。設定 `down: false` 即可恢復。
- **刪除** 會移除啟用連結、網站檔案與上游檔案，重新載入，並刪除記錄。憑證記錄與檔案會保留，因此重新建立 Ingress 時不會觸發 ACME 速率限制，除非指定 `purge_cert=1`。

## 狀態

`phase` 用於彙整各個 condition：

| 階段 | 時機 |
|---|---|
| `Pending` | `observed_generation < generation`，且 worker 尚未處理 |
| `Applying` | worker 正在套用 |
| `Ready` | `ConfigApplied`、`CertReady`（或 TLS 模式為 `none`）與 `BackendsHealthy` 皆為 true |
| `Degraded` | 設定已套用，但憑證尚未就緒，或某個 backend 沒有未被 down 且健康的 server |
| `Error` | 上一次套用失敗；先前的設定仍在運作 |

Condition 的 reason：

| Condition | Reasons |
|---|---|
| `ConfigApplied` | `Applied`、`TestFailed`、`ReloadFailed`、`RenderFailed`、`Drifted`（由調和重寫） |
| `CertReady` | `Issued`、`Existing`、`NotRequired`、`Issuing`、`DNSNotReady`、`IssueFailed`、`CertNotFound`、`Expired` |
| `BackendsHealthy` | `AllHealthy`、`PartiallyHealthy`、`NoHealthyServer`、`Unknown` |

### 憑證

- `acme`：在憑證存在之前，只會渲染 HTTP 監聽與驗證 location。Nginx UI 會先檢查每個主機是否都解析到這台伺服器（否則回報 `DNSNotReady`，並以 1 到 30 分鐘的退避間隔重試），接著以預設或指定的 ACME 使用者簽發憑證，並啟用自動續期。簽發成功後才會渲染 HTTPS 監聽。續期由一般的自動憑證工作負責。
- `cert`：直接使用 `cert_id` 指定的憑證；它必須涵蓋每個主機。
- `none`：僅 HTTP。

調和會每隔 `[ingress] ReconcileIntervalMinutes`（預設為 5）執行一次。它會重寫內容已與 spec 不符的產生檔案，並更新 `cert_not_after` 與 server 健康狀態。

## 在介面中檢視 Ingress

側邊選單中的 **Ingresses** 會列出所有 Ingress，包含其擁有者、主機、階段、憑證到期時間、backend 健康狀態（健康的 server 數量相對於全部 server，以及有多少已以 `down` 摘除），以及已套用的版本與最新版本（`gen 2/3` 表示版本 3 仍在等待套用）。可依擁有者篩選，或依名稱、主機或外部 ID 搜尋。頁面開啟期間，列表會每隔數秒自動重新整理。

詳細資料抽屜會以易讀的形式顯示 spec（主機、附帶各 server 健康狀態的 backend、路由、TLS 與 policy）、各 condition 及其 reason 與訊息（包含套用失敗時的 `nginx -t` 輸出），以及產生的網站設定。可進行兩項操作：

- **立即調和** 會將該 Ingress 排入調和流程（`POST /ingresses/:name/reconcile`）。
- **刪除** 會以 `force=1` 與其擁有者移除該 Ingress，並可選擇一併移除其憑證。擁有它的控制器通常會在下一次同步時重新建立，因此若要永久刪除，請從控制器中移除。

除此之外，Ingress 在介面中為唯讀：請透過擁有它的控制器進行變更。

產生的網站在網站列表中會帶有 **Ingress** 標籤，並在網站編輯器中顯示連回該 Ingress 的警告。這類網站的儲存、重新命名、啟用、停用、進入維護模式與刪除皆會被鎖定。請在編輯器中開啟 **仍然編輯**（或勾選網站列表批次工具列中的核取方塊），即可帶著強制旗標送出變更；下一次調和仍會將其覆寫。

## 設定

```ini
[ingress]
AllowedBackends = 127.0.0.1/32
AllowedBackends = ::1/128
ReconcileIntervalMinutes = 5
```

`AllowedBackends` 是受保護的設定：只能在 `app.ini` 或環境變數（`NGINX_UI_INGRESS_ALLOWED_BACKENDS`）中變更，無法透過 API 修改。

## 錯誤

錯誤使用一般的回應主體 `{ "scope": "ingress", "code": 40901, "message": "…", "params": [...] }`。與大多數 Nginx UI 端點不同，Ingress 端點會以代碼前三位數字所代表的 HTTP 狀態回應，因此控制器只需依賴狀態碼即可。若要做特定處理，請比對 `scope` 與 `code`。

Scope `ingress`：

| 代碼 | HTTP | 意義 |
|---|---|---|
| 40001 | 400 | Ingress 名稱無效 |
| 40002 | 400 | 擁有者無效 |
| 40003 | 400 | 主機無效 |
| 40004 | 400 | backend 無效 |
| 40005 | 400 | backend server 超出允許的範圍 |
| 40006 | 400 | 路由無效 |
| 40007 | 400 | tls 無效 |
| 40008 | 400 | policy 無效 |
| 40009 | 404 | 在 backend 中找不到 server |
| 40401 | 404 | 找不到 Ingress |
| 40801 | 408 | 等待逾時（變更仍保留在佇列中） |
| 40901 | 409 | 由其他擁有者持有 |
| 40902 | 409 | 主機已由其他 Ingress 提供服務 |
| 40903 | 409 | 已存在與產生名稱相同、且不受此 Ingress 管理的網站 |
| 50001 | 500 | 套用失敗 |
| 50002 | 500 | 憑證尚未就緒 |

Scope `site`：`40901`（409）該網站由 Ingress 管理。
