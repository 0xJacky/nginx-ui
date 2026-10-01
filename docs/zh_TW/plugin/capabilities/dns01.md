---
outline: [2, 3]
---

# DNS-01

`dns01` 外掛為某個 DNS 服務商完成 ACME DNS-01 驗證：簽發憑證時透過服務商的 API 發佈 `_acme-challenge` TXT 記錄，完成後再刪除它。它的服務商會出現在 Nginx UI 的憑證表單和 DNS 認證編輯器中。

| 方法 | 必要 | 用途 |
| --- | --- | --- |
| `dns01.present` | 是 | 發佈驗證用的 TXT 記錄。 |
| `dns01.cleanup` | 是 | 刪除 `present` 發佈的內容。 |
| `dns01.validate` | 否 | 在不發佈任何內容的情況下檢查認證。 |
| `dns01.options` | 否 | 回報服務商的傳播時間。 |
| `dns01.check` | 否 | 回報記錄是否已傳播。 |

外掛沒有實作的選用方法回覆 `-32002`（不支援），Nginx UI 會改用自己的行為。

## 宣告服務商 {#declaring-providers}

```json [plugin.json]
"capabilities": ["dns01"],
"permissions": ["network"],
"dns01": {
  "providers": [
    {
      "name": "MyDNS",
      "code": "mydns",
      "links": { "api": "https://mydns.example/docs/api" },
      "propagation_timeout_seconds": 120,
      "polling_interval_seconds": 2,
      "form": {
        "fields": [
          { "key": "MYDNS_API_TOKEN", "label": "API token", "group": "credential", "secret": true },
          { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
        ]
      }
    }
  ]
}
```

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `name` | 是 | DNS 服務商的名稱。 |
| `code` | 是 | 服務商的識別碼，參見[命名規則](../naming.md#provider-and-kind-codes)。在清單內唯一。 |
| `links.api` | 否 | 服務商 API 文件的連結。 |
| `propagation_timeout_seconds` | 否 | Nginx UI 等待記錄傳播的時長。 |
| `polling_interval_seconds` | 否 | 檢查傳播的間隔。 |
| `form` | 是 | 服務商接受的值。參見[認證表單](#credential-form)。 |

## 發佈記錄 {#publishing-the-record}

```json
{
  "jsonrpc": "2.0", "id": 10, "method": "dns01.present",
  "params": {
    "provider": "mydns",
    "config": { "MYDNS_API_TOKEN": "tok_live_xxx", "MYDNS_TTL": "120" },
    "options": { "credential_id": "7", "disable_cname": false },
    "domain": "example.com",
    "fqdn": "_acme-challenge.example.com.",
    "effective_fqdn": "_acme-challenge.example.com.",
    "value": "gfj9Xq...Rg85nM",
    "token": "evaGxfADs...62jcerQ",
    "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7",
    "dry_run": false
  }
}
```

| 欄位 | 意義 |
| --- | --- |
| `provider` | 服務商的 `code`。 |
| `config` | 認證的值：表單兩組欄位合併成的一個對應，以欄位 `key` 為鍵。 |
| `options` | 憑證在驗證表單中的選項。參見[憑證選項](#certificate-options)。 |
| `domain` | 正在驗證的網域名稱，不帶開頭的 `*.`。 |
| `fqdn` | `_acme-challenge` 名稱。 |
| `effective_fqdn` | 記錄的實際位置：跟隨了 CNAME 時為 CNAME 的目標，否則等於 `fqdn`。 |
| `value` | TXT 記錄的值。 |
| `token`、`key_auth` | ACME 驗證權杖和金鑰授權，供需要自行計算 `value` 的外掛使用。 |
| `dry_run` | 為 `true` 時只檢查輸入，不要存取服務商，也不要修改任何記錄。 |

記錄發佈後回覆 `{}`。失敗時回覆錯誤：

- 原因是使用者輸入的內容（例如權杖被拒絕）時，回覆 `-32003`，並用 `data.field` 指出欄位；
- 其他情況（例如服務商故障）回覆 `-32000`。

::: warning 注意
絕不要讓 `config` 中的值出現在日誌或錯誤訊息中。
:::

## 刪除記錄 {#removing-the-record}

`dns01.cleanup` 的參數與 `dns01.present` 相同，刪除為同一 `effective_fqdn` 和 `value` 發佈的記錄，並回覆 `{}`。簽發結束時 Nginx UI 總會呼叫它，所以在沒有可刪除的內容時（包括 `present` 從未執行或已失敗時）它也必須成功。

## 驗證認證 {#validating-a-credential}

```json
{ "jsonrpc": "2.0", "id": 11, "method": "dns01.validate", "params": { "provider": "mydns", "config": { "MYDNS_API_TOKEN": "" } } }
```

在不存取服務商的情況下檢查這些值：對第一個缺少或格式錯誤的欄位回覆帶 `data.field` 的 `-32003`，值看起來可用時回覆 `{}`。使用者填寫認證表單時 Nginx UI 會呼叫它。

## 傳播時間 {#propagation-timing}

::: code-group

```json [請求]
{ "jsonrpc": "2.0", "id": 12, "method": "dns01.options", "params": { "provider": "mydns", "config": {}, "options": {} } }
```

```json [回應]
{ "jsonrpc": "2.0", "id": 12, "result": { "propagation_timeout_seconds": 120, "polling_interval_seconds": 2, "sequential_interval_seconds": 0 } }
```
:::

`sequential_interval_seconds` 不為 `0` 時，表示服務商無法同時處理同一帳戶的兩個驗證，呼叫之間必須至少間隔這麼多秒。沒有這個方法時，Nginx UI 使用清單中的時間，再退回到自己的預設值。

## 檢查傳播 {#checking-propagation}

::: code-group

```json [請求]
{
  "jsonrpc": "2.0", "id": 13, "method": "dns01.check",
  "params": { "provider": "mydns", "config": {}, "options": {}, "domain": "example.com", "fqdn": "_acme-challenge.example.com.", "value": "gfj9Xq...Rg85nM", "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7" }
}
```

```json [回應]
{ "jsonrpc": "2.0", "id": 13, "result": { "ready": false, "effective_fqdn": "_acme-challenge.example.com.", "detail": "NXDOMAIN from ns1.example.com" } }
```
:::

`ready` 表示記錄是否在外掛檢查的所有位置都已可見，`effective_fqdn` 是它查詢的名稱，`detail` 說明尚未就緒的原因。當外掛能比公共 DNS 更快或更可靠地檢查時（例如透過服務商的 API），才需要實作這個方法。沒有它時，Nginx UI 會自己查詢 DNS。

## 憑證選項 {#certificate-options}

`options` 攜帶驗證表單與憑證一起儲存的內容。它對 Nginx UI 是不透明的：外掛的[驗證表單元件](../slots.md#certificate-challenge-form)寫入 `challenge_config` 的內容會原樣傳回。官方 DNS-01 外掛使用：

| 鍵 | 意義 |
| --- | --- |
| `credential_id` | 所選認證的 ID。 |
| `disable_cname` | 不跟隨驗證記錄的 CNAME。 |
| `disable_authoritative_ns_propagation` | 略過對權威名稱伺服器的檢查。 |
| `disable_recursive_ns_propagation` | 略過對遞迴名稱伺服器的檢查。 |

`options` 可能為空，例如外掛還沒有表單時簽發的憑證。此時請使用合理的預設值，而不是直接失敗。

## 認證表單 {#credential-form}

`form` 描述服務商接受的每個值。Nginx UI 只根據它建立認證表單，絕不會儲存或傳送表單沒有列出的鍵。沒有任何值的服務商宣告 `"fields": []`。

```json [plugin.json]
"form": {
  "fields": [
    { "key": "MYDNS_API_TOKEN", "label": "API token", "help": "Needs the DNS edit permission.", "group": "credential", "secret": true },
    { "key": "MYDNS_API_EMAIL", "label": "Account email", "group": "credential" },
    { "key": "MYDNS_API_KEY", "label": "API key", "group": "credential", "secret": true },
    { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
  ],
  "methods": [
    { "name": "API token", "recommended": true, "fields": ["MYDNS_API_TOKEN"] },
    { "name": "Global API key", "fields": ["MYDNS_API_EMAIL", "MYDNS_API_KEY"] },
    { "name": "Instance role", "fields": [], "values": { "MYDNS_AUTH_MODE": "instance" } }
  ]
}
```

### 欄位 {#fields}

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `key` | 是 | 值在 `config` 中的鍵，在表單內唯一。 |
| `label` | 是 | 簡短的標籤。 |
| `help` | 否 | 顯示在輸入框下方的一句說明。 |
| `group` | 是 | `credential` 表示登入用的值，`setting` 表示調校項目，例如 TTL、基礎網址或逾時。 |
| `optional` | 否 | 沒有這個值服務商也能運作。 |
| `secret` | 否 | 密碼、權杖或金鑰，顯示為密碼輸入框。 |
| `default` | 否 | 欄位為空時使用的值。顯示為預留位置內容，不會替使用者儲存。 |
| `unit` | 否 | 秒數時為 `seconds`。 |
| `link` | 否 | 欄位的文件連結。 |

Nginx UI 會把 `setting` 欄位與認證分開顯示，並分開儲存兩組值，其中認證以機密處理。外掛收到的 `config` 中兩組值是合併在一起的。為空或為 `false` 的屬性請省略，以保持清單簡潔。

### 登入方式 {#sign-in-methods}

服務商提供多種登入方式時，`methods` 列出它們，至少兩項：

| 欄位 | 必填 | 意義 |
| --- | --- | --- |
| `name` | 是 | 方式的名稱，唯一。 |
| `recommended` | 否 | 預設選取這種方式，最多一項。 |
| `fields` | 是 | 這種方式使用的 `credential` 欄位的鍵。不需要輸入的方式為空。 |
| `values` | 否 | 在外掛中選定這種方式的固定值，例如認證模式。 |

- 沒有被任何方式列出的認證欄位是共用的，每種方式下都會顯示。Nginx UI 只顯示所選方式的欄位和共用欄位，儲存時清除所選方式不使用的認證值。
- `values` 會與認證一起儲存，並在 `config` 中傳送。`values` 的鍵通常不是欄位；它也可以是某些方式列出、另一些方式固定取值的認證欄位：列出它的方式顯示該欄位，設定它的方式固定它的值。不能有一種方式既列出又設定同一個鍵。
- 任意兩種方式的欄位和值不能完全相同。

編輯已儲存的認證時，Nginx UI 預先選取 `values` 與儲存值相符、且欄位都有值的方式；否則選取推薦的方式；再否則選取第一種。

### 翻譯 {#translations}

服務商 `name`、每個 `label` 和 `help` 以及每種方式的 `name` 都是英文原文。Nginx UI 會用外掛瀏覽器套件透過 `registerTranslations` 註冊的翻譯來翻譯它們，沒有翻譯時顯示英文。

違反本節任何規則的表單無法通過 [`dns01-form`](../rules.md#dns01-form) 檢查，Nginx UI 可能會拒絕安裝該外掛。
