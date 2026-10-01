---
outline: [2, 3]
---

# 插槽

插槽是 Nginx UI 介面的擴充點。瀏覽器套件用 `registry.registerSlot` 把元件掛載到插槽中：

```ts
registry.registerSlot('certificate.issue.footer', IssueHint, {
  order: 10,
  when: ctx => ctx.options.challenge_method === 'dns01',
})
```

| 選項 | 意義 |
| --- | --- |
| `order` | 多個外掛使用同一插槽時，值越小越先渲染。 |
| `when(ctx)` | 回傳 `false` 時，在該上下文中不渲染。 |
| `label` | 需要顯示文字的插槽使用的標籤，是一段英文文字，會用外掛的翻譯進行翻譯。 |
| `sortValue`、`filters` | 日誌清單欄的排序和篩選。 |

## 屬性 {#props}

掛載的元件會收到兩份插槽上下文：展開為個別的屬性，以及一個 `context` 屬性。以下兩種寫法都可以：

```ts
defineProps<{ options: CertificateOptions }>()
defineProps<{ context: { options: CertificateOptions } }>()
```

每個掛載的元件都有獨立的錯誤邊界。元件擲出例外時只顯示一則行內錯誤，不會破壞頁面或同一插槽中的其他元件。

## 可用的插槽 {#available-slots}

| 插槽 | 上下文 | 位置 |
| --- | --- | --- |
| `certificate.challenge.form:{method}` | `{ options }` | 某種 ACME 驗證方式（例如 `dns01`）的設定。 |
| `certificate.issue.footer` | `{ options }` | 憑證簽發表單底部。 |
| `dns.credential.form:{provider_code}` | `{ credential, provider }` | 某個服務商的 DNS 認證表單中的額外欄位。 |
| `dns.credential.hint:{provider_code}` | `{ credential, provider }` | 服務商認證欄位上方的內容。 |
| `plugin.settings:{plugin_id}` | `{ settings }` | 另一個外掛的設定中。 |
| `sidebar.footer` | `{}` | 側邊欄底部。 |
| `nginx_log.view:{key}` | `{ path, type }` | 某個日誌檔案的額外檢視。 |
| `nginx_log.list.toolbar` | `{ type }` | 日誌清單上方的操作區。 |
| `nginx_log.list.column:{key}` | `{ row }` | 日誌清單的額外欄。 |
| `nginx_log.list.row.actions` | `{ row }` | 日誌清單中某一列的操作。 |
| `site.log.actions` | 參見[網站日誌操作](#site-log-actions) | 某個網站的日誌操作。 |

對 Nginx UI 不認識的插槽名稱的註冊會被忽略，將來的版本可能會新增插槽。外掛為自己定義的插槽以外掛 ID 為前綴，參見[命名規則](./naming.md#settings-keys-and-slot-names)。

### 憑證驗證表單 {#certificate-challenge-form}

`certificate.challenge.form:{method}` 收到 `options`，即正在簽發的憑證的響應式選項。元件讀寫 `options.challenge_config`，這個物件屬於外掛：Nginx UI 會把它和憑證一起儲存，並在每次 DNS-01 呼叫中原樣作為 `options` 交給外掛。參見 [DNS-01](./capabilities/dns01.md#certificate-options)。

## 日誌頁面插槽 {#log-page-slots}

### 檢視 {#views}

每個 `nginx_log.view:{key}` 註冊都會在日誌頁面的切換器中、內建的原始檢視旁邊加入一種模式，標籤為 `opts.label`（沒有時使用鍵名）。頁面把所選模式保存在 `view` 查詢參數中，因此 `?view=<key>` 可以直接連結到它。只有所選模式的元件會被掛載，並收到檔案的 `path` 和 `type`（`access` 或 `error`）。`opts.when(ctx)` 決定是否為某個檔案提供該模式。

### 欄 {#columns}

每個 `nginx_log.list.column:{key}` 註冊都會在內建欄之後加入一欄，依 `opts.order` 排序，標題為 `opts.label`。元件渲染它收到的那一列的儲存格。

`opts.when(ctx)` 對每份清單只呼叫一次，參數為 `{ type }` 而不是某一列，它決定這一欄是否存在。只適用於存取日誌的欄只需註冊一次，並在錯誤日誌清單中隱藏：

```ts
registry.registerSlot('nginx_log.list.column:index_status', StatusCell, {
  label: 'Index Status',
  when: ctx => ctx.type === 'access',
  sortValue: row => statusRank(row.path),
  filters: [{ label: 'Indexed', value: 'indexed', match: row => isIndexed(row.path) }],
})
```

Nginx UI 一次載入整份清單，並在瀏覽器中對外掛欄進行排序和篩選：

- 提供 `sortValue(row)` 時，標頭依回傳值排序：數字依大小，字串依地區設定順序，`null` 和 `undefined` 排在最後。
- 提供 `filters` 時，標頭為每一項提供一個選項，標籤為 `label`。同一欄中選取的選項以「或」組合，不同欄之間以「且」組合。`value` 識別選項。

每一列至少包含 `path`、`type`、`name` 和 `config_file`。請忽略不認識的欄位。

### 工具列與列操作 {#toolbar-and-row-actions}

`nginx_log.list.toolbar` 和 `nginx_log.list.row.actions` 依 `opts.order` 渲染每個註冊。`opts.when` 收到與元件相同的上下文。

### 網站日誌操作 {#site-log-actions}

`site.log.actions` 出現在網站編輯器和網站清單中，收到某個網站的日誌檔案：

| 屬性 | 意義 |
| --- | --- |
| `siteName` | 網站。 |
| `accessLogPath`、`errorLogPath` | 網站自己的 `access_log` 或 `error_log` 路徑；沒有時為網站退回使用的 nginx 預設日誌；都沒有時為空字串。 |
| `accessLogInherited`、`errorLogInherited` | 路徑是 nginx 預設日誌而不是網站自己的日誌時為 `true`。 |

預設日誌中可能包含其他網站的流量。針對單一網站流量的操作，在路徑是繼承來的時候應隱藏自己，或者明確說明這一點。
