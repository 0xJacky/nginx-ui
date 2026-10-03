---
outline: [2, 3]
---

# 範本與翻譯

外掛可以提供內容來取代程式碼，或與程式碼一起提供：nginx 設定範本和翻譯檔案。內容在清單的 `content` 區塊中以外掛套件內目錄的形式宣告：

```json [plugin.json]
{
  "id": "io.github.example.snippets",
  "name": "Extra snippets",
  "version": "1.0.0",
  "api_version": 1,
  "content": {
    "templates": "templates",
    "locales": "locales"
  }
}
```

```text
io.github.example.snippets/
├── plugin.json
├── README.md
├── LICENSE
├── templates/
│   ├── block/
│   │   └── cache-static.conf
│   └── conf/
│       └── ghost.conf
└── locales/
    ├── de_DE.po
    └── zh_CN.po
```

內容就是資料：其中任何內容都不會作為程式執行。只有 `content` 的外掛沒有進程，啟用和停用它只會加入和移除它提供的內容。這類外掛不能宣告 `capabilities`、`cron` 或 `events`，因為它們由進程提供。內容也可以與 `server` 和 `webapp` 同時存在。

## 範本 {#templates}

`content.templates` 是一個最多包含兩個子目錄的目錄，與內建範本的兩份清單對應：

- `conf/` 用於整個 server 的範本；
- `block/` 用於使用者加入 server 中的片段。

其中至少一個包含範本。範本是檔名符合 `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$` 的檔案；其他項目會被忽略，檢查工具會對它們發出警告。

### 格式 {#format}

範本使用與內建範本相同的格式：

```text
# Nginx UI Template Start
name = "Cache static files"
author = "@example"
description = { en = "Cache images, scripts and styles", de_DE = "Bilder, Skripte und Stylesheets zwischenspeichern" }

[variables.expires]
type = "string"
name = { en = "Expires", de_DE = "Ablauf" }
value = "30d"
# Nginx UI Template End
location ~* \.(?:css|js|png|jpe?g|gif|svg|webp)$ {
    expires {{.expires}};
    add_header Cache-Control "public";
}
```

- `# Nginx UI Template Start` 與 `# Nginx UI Template End` 之間的標頭是 TOML，包含 `name`、`author`、`description`（各語言的文字）和 `variables`。請為每個範本設定 `name`，沒有時會顯示檔名。
- 每個變數有 `type`（`string`、`boolean` 或 `select`）、各語言的 `name`、預設值 `value`，`select` 型別還有把每個選項對應到其標籤的 `mask`。
- 結束行之後的所有內容是範本主體：一個 Go [text/template](https://pkg.go.dev/text/template)，輸出在 `server` 區塊中有效的 nginx 指令。`# Nginx UI Custom Start` 與 `# Nginx UI Custom End` 之間的選用部分會以同樣方式渲染，並加入 server 的自訂指令。
- 範本資料是以鍵區分的每個變數，以及 `HTTPPORT` 和 `HTTP01PORT`，即 Nginx UI 監聽的連接埠。

::: v-pre
範本可以使用 `{{.name}}`、`if`、`else` 和 `with`，以及函式 `and`、`or`、`not`、`eq`、`ne`、`lt`、`le`、`gt`、`ge`、`len`、`index`、`print`、`println`、`html`、`js` 和 `urlquery`。其他任何寫法（`range`、`define`、`template`、`block`、`printf`、`call`）都會被拒絕，因為迴圈和格式寬度可能讓渲染失去邊界。範本檔案最大 256 KiB，渲染後的每部分最大 1 MiB。
:::

### 驗證 {#validation}

Nginx UI 在安裝或更新外掛前會驗證每個範本。缺少開始或結束行、標頭無效、不是有效的範本、用預設值渲染失敗或渲染出無效 nginx 語法的範本，都會導致外掛套件被拒絕。

外掛的範本與內建範本出現在同樣的清單中，並標明來自哪個外掛。它們永遠不會取代內建範本或其他外掛的範本，並在外掛停用後消失。只有使用者選擇某個範本並看到它渲染出的內容後，範本的指令才會加入設定。

## 翻譯 {#translations}

`content.locales` 是一個包含 GNU gettext PO 檔案的目錄，每種語言一個檔案，命名為 `<lang>.po`，其中 `<lang>` 是 Nginx UI 介面的語言代碼，例如 `zh_CN` 或 `de_DE`。只需提供你翻譯了的語言。目錄至少包含一個這樣的檔案；其他項目會被忽略，檢查工具會對它們發出警告。檔名不是 Nginx UI 支援的語言代碼時，[`nginx-ui plugin lint`](../rules.md#content-locales) 會回報它，並列出所有可用的代碼。

檔案使用 UTF-8，以標頭項目（`msgid ""`）開始，每個 `msgid` 都有 `msgstr`。標記為 `fuzzy` 的項目會被忽略。包含無法解析的檔案的外掛套件會被 Nginx UI 拒絕。

外掛啟用期間，它的項目會加入 Nginx UI 為該語言提供的翻譯，因此外掛透過 Nginx UI 的 gettext 顯示的文字（瀏覽器套件、範本中的名稱）不需額外工作即可翻譯。外掛不能修改 Nginx UI 本身的翻譯：Nginx UI 已翻譯的文字以它的翻譯為準。多個外掛翻譯同一段文字時，以外掛 ID 最小的外掛為準。

翻譯只取代顯示文字，並且一律被當作文字處理，絕不會當作標記。
