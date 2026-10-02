---
outline: [2, 3]
---

# 外掛目錄

Nginx UI 的外掛市集列出一個或多個**外掛目錄**中的外掛。外掛目錄是描述外掛及其外掛套件位置的靜態 JSON 文件。官方外掛目錄位於 `https://plugins.nginxui.com`，任何人都可以發佈自己的外掛目錄，供其他人加入為來源。

## 發佈到官方外掛目錄 {#publishing-in-the-official-catalog}

官方外掛目錄由 [nginxui/plugins](https://github.com/nginxui/plugins) 儲存庫建置。提交外掛的步驟：

1. 在公開儲存庫中發佈外掛，並建立一個以簽章後的外掛套件為附件的 GitHub Release。
2. 確保 `nginx-ui plugin lint`，以及（對有伺服器端進程的外掛）`nginx-ui plugin conformance` 能夠通過。儲存庫會對你的版本執行這兩項檢查。
3. 使用 **Submit a plugin** 表單建立 issue，它會為你草擬項目；或者直接提交一個加入 `plugins/<外掛 ID>.json` 的拉取請求。

儲存庫的貢獻指南介紹了審查流程。之後的新版本會自動從你的儲存庫取得。

## 目錄文件 {#catalog-document}

```json [index.json]
{
  "schema_version": 1,
  "name": { "en": "Example Plugins", "zh_TW": "範例外掛" },
  "icon": "https://plugins.example.com/assets/icon.png",
  "updated_at": "2026-09-30T00:00:00Z",
  "plugins": [ ... ]
}
```

| 欄位 | 意義 |
| --- | --- |
| `schema_version` | `1`。 |
| `name` | 各語言的目錄名稱，以 `en` 為備援。Nginx UI 在任何提到外掛來源的地方都顯示它。 |
| `icon` | 代表該目錄的方形圖片。 |
| `updated_at` | 此版本文件的時間。 |
| `plugins` | 項目。 |

[JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/catalog.schema.json) 描述了所有成員。不認識的成員會被忽略。

### 部署位置 {#where-to-serve-it}

把文件放在網站的 `/v1/index.json`，例如 `https://plugins.example.com/v1/index.json`。這樣使用者只需輸入 `plugins.example.com` 即可加入來源：Nginx UI 會依序在 `/v1/index.json` 和 `/index.json` 尋找目錄，並保留有回應的網址。

### 名稱與圖示 {#name-and-icon}

目錄自己決定自己的名稱。Nginx UI 在外掛市集和來源清單中顯示它，會合併多餘的空白，並可能把它截短到 64 個字元。沒有名稱時顯示網址中的主機。

圖示是 PNG、SVG 或 WebP 圖片，方形，除向量圖外寬度至少 64 像素。它依照[螢幕截圖](#screenshots)的規則載入，只能來自目錄自己的主機或 GitHub；其他位置的圖示會被捨棄，並顯示通用圖片。

## 項目 {#entries}

```json [index.json]
{
  "id": "io.github.example.mydns",
  "name": { "en": "MyDNS" },
  "description": { "en": "DNS-01 challenges through the MyDNS API." },
  "author": "example",
  "author_public_key": "RWQ...",
  "repository_url": "https://github.com/example/mydns",
  "readme_url": "https://raw.githubusercontent.com/example/mydns/main/README.md",
  "icon_url": "https://raw.githubusercontent.com/example/mydns/main/icon.png",
  "categories": ["dns01"],
  "capabilities": ["dns01"],
  "license": "MIT",
  "trust": "community",
  "stage": "production",
  "releases": [ ... ]
}
```

| 欄位 | 意義 |
| --- | --- |
| `id` | 外掛 ID。 |
| `name`、`description` | 各語言的文字，必須包含 `en`。 |
| `author` | 外掛的發佈者。 |
| `author_public_key`、`trust` | 參見[發佈者](#publisher)。 |
| `homepage_url`、`repository_url`、`readme_url`、`icon_url` | 連結。README 會顯示在外掛詳細資料中。 |
| `screenshots` | 參見[螢幕截圖](#screenshots)。 |
| `categories`、`capabilities` | 用於篩選外掛市集。 |
| `license` | 外掛授權條款的 SPDX 識別碼。 |
| `stage` | `production` 或 `beta`，顯示在外掛旁邊的標籤。 |
| `provides` | 選用。最新版本提供的內容。`dns01.since` 是外掛開始提供 DNS-01 的版本，`dns01.providers` 列出它的 DNS 服務商，每項包含 `code` 和 `name`；後來才加入的服務商帶有自己的 `since`；被較新版本移除、但最新穩定版仍包含的服務商帶有 `removed_in`。版本滿足 `since` ≤ 版本 < `removed_in` 時包含該服務商。讓 Nginx UI 在安裝之前就能找到外掛。 |
| `releases` | 各個版本，見下文。 |

項目的連結和圖片只使用目錄主機、外掛套件主機或 GitHub 上的 `https` 網址。

## 版本 {#releases}

```json [index.json]
{
  "version": "1.2.0",
  "released_at": "2026-09-30T00:00:00Z",
  "api_version": 1,
  "min_nginx_ui_version": "2.7.0",
  "platforms": ["linux-amd64", "linux-arm64"],
  "downloads": {
    "linux-amd64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-amd64.tar.gz", "sha256": "..." },
    "linux-arm64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-arm64.tar.gz", "sha256": "..." }
  },
  "release_notes_url": "https://github.com/example/mydns/releases/tag/v1.2.0",
  "notes": "### Features\n\n- Support the new provider API",
  "manifest": { ... }
}
```

| 欄位 | 意義 |
| --- | --- |
| `version` | 該版本的外掛版本號。 |
| `api_version` | 協定世代，`1`。 |
| `min_nginx_ui_version` | 與清單中的意義相同。 |
| `platforms` | 該版本可以安裝的平台：`downloads` 的所有鍵加上通用外掛套件支援的平台。為空表示所有平台。 |
| `downloads` | 依平台鍵列出的外掛套件，`any` 表示可在所有平台執行的外掛套件。每項包含 `url` 和 `sha256`。 |
| `download_url`、`sha256` | 通用外掛套件，`downloads` 未列出的平台都使用它。 |
| `channel` | `stable`、`beta` 或 `dev`。參見[發佈通道](#release-channels)。 |
| `release_notes_url` | 該版本完整發佈說明的頁面。 |
| `notes` | 該版本的變更，Markdown 格式，最多 4096 個字元。Nginx UI 會在外掛詳細資訊頁顯示它；有更新時，顯示自已安裝版本以來每個版本的說明。 |
| `yanked` | 該版本已撤回，不再提供。 |
| `manifest` | 該版本 `plugin.json` 的快照，見下文。 |

版本至少包含 `downloads` 和 `download_url` 中的一個。`downloads` 中某個平台的項目指向該平台的外掛套件，`any` 項目指向可在所有平台執行的外掛套件。請為每個下載提供 `sha256`。

### Nginx UI 如何選擇外掛套件 {#how-nginx-ui-picks-a-package}

在平台 `P` 上，Nginx UI 依序選擇 `downloads[P]`、`downloads["any"]`，以及在 `platforms` 為空或包含 `P` 或 `any` 時的 `download_url`。都不符合時，該版本無法在 `P` 上安裝。它在所有場合都使用同樣的選擇方式：選擇最新版本、檢查更新、解析相依，以及為其他節點或離線安裝下載外掛套件。

下載後，它先檢查 `sha256`，再檢查[簽章](./signing.md)，解壓縮後再確認清單的 ID 和版本與項目和版本一致。檢查碼只說明下載到的是目錄所指的檔案，只有簽章才能說明是誰發佈了它。

### 清單快照 {#the-manifest-snapshot}

`manifest` 包含該版本 `plugin.json` 中 Nginx UI 在下載任何內容之前要讀取的欄位：`id`、`name`、`version`、`description`、`i18n`、`homepage_url`、`api_version`、`min_nginx_ui_version`、`server`、`capabilities`、`permissions`、`requires`、`requires_capabilities`、`conflicts` 和 `network_hosts`。它的 `server.executables` 列出該版本提供的所有平台，儘管每個單一平台的外掛套件只宣告自己的平台。DNS-01 服務商清單這類能力設定區塊留在外掛套件裡：它們隨版本變化，而 Nginx UI 安裝時讀取的是外掛套件自己的 `plugin.json`。

## 發佈者 {#publisher}

| 欄位 | 意義 |
| --- | --- |
| `author_public_key` | 為外掛套件簽章的 minisign 公鑰，即 `.pub` 檔案中的 base64 那一行。從該項目下載並由它簽署的外掛套件是 `community`。 |
| `trust` | `official`、`verified` 或 `community`：目錄預期的等級。僅用於列出和篩選，從不授予等級。只有保留命名空間 `com.nginxui.*` 中的 ID 才會顯示為 `official`，其他 ID 顯示為 `community`。 |

`author_public_key` 只對從攜帶它的項目下載的外掛套件有效。參見[簽章與信任](./signing.md#trust-levels)。

## 發佈通道 {#release-channels}

每個版本都屬於一個通道，從最穩定到最不穩定依序為：`stable`、`beta` 和 `dev`。沒有 `channel` 成員時，由版本號決定：

- 沒有預發佈部分的版本是 `stable`；
- 預發佈部分的第一個識別碼為 `alpha`、`dev`、`nightly`、`snapshot`、`canary` 或 `preview`（不區分大小寫）的版本是 `dev`，因此 `1.0.0-nightly.20260930` 是 `dev`，而 `1.0.0-alphabet` 不是；
- 其他預發佈版本，例如 `1.0.0-beta.1` 或 `2.0.0-rc.1`，是 `beta`。

設定 `channel` 可以把一般版本號放到較不穩定的通道。項目的 `stage` 不會改變任何通道。

::: tip 提示
請用點分隔的數字為預發佈版本編號，例如 `1.1.0-beta.10`：只有單獨成段時識別碼才依數字比較，所以 `1.1.0-beta10` 會排在 `1.1.0-beta9` 之前。
:::

使用者如何收到版本：

- 每個已安裝的外掛跟隨一個通道，除非使用者選擇其他通道，否則為 `stable`。更新來自所跟隨的通道，或者在已安裝版本的通道更不穩定時來自該通道，並且包含所有更穩定的版本。跟隨 `beta` 的使用者也會收到結束一系列測試版的正式版。
- 安裝單一測試版不會改變所跟隨的通道。這樣的安裝會收到更新的測試版，直到安裝了正式版，然後回到穩定通道。只有測試版的外掛也是如此：使用者先得到測試版，然後得到第一個正式版。
- 對於尚未安裝的外掛，Nginx UI 選擇最新的穩定版，沒有時選擇最新的 `beta`，再沒有時選擇最新的 `dev`，並標記沒有穩定版的外掛。
- 任何未撤回且能在本機執行的版本都可以依版本號安裝。安裝較舊的版本屬於降級，只有在使用者要求時才會發生，並會警告新版本寫入的資料可能無法被舊版本讀取。

## 螢幕截圖 {#screenshots}

```json [index.json]
"screenshots": [
  {
    "url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials.png",
    "dark_url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials-dark.png",
    "caption": { "en": "The credential form" }
  }
]
```

最多八張外掛使用時的 PNG、JPEG 或 WebP 圖片，依顯示順序排列，每張可以附帶各語言的說明。寬度 1280 到 1920 像素、寬高比約 16:10 的圖片適合所有螢幕。Nginx UI 只從目錄主機、外掛套件主機或 GitHub 上的 `https` 網址載入它們，捨棄其餘圖片，並且永遠不會把圖片缺少視為項目的問題。

`dark_url` 是選用的，內容是同一畫面的深色主題版本。介面為深色時 Nginx UI 顯示它，否則顯示 `url` 的圖片；沒有深色圖片，或深色圖片無法載入時，兩種主題都顯示淺色圖片。兩張圖片應拍攝同一畫面，尺寸相同。

## 託管自己的外掛目錄 {#hosting-a-catalog-of-your-own}

外掛目錄是靜態檔案，任何 Web 伺服器或靜態託管服務都可以：

- 透過 HTTPS 在 `/v1/index.json` 提供它。
- 把外掛套件、圖片和 README 放在同一主機或 GitHub 上。
- 為目錄設定 `name` 和 `icon`。
- 為外掛套件簽章：在開發者模式之外，未簽章的外掛套件永遠無法安裝。目錄中的 `author_public_key` 讓你的外掛套件成為 `community`，維運人員還需要允許社群外掛。

使用者在 **外掛 > 市集 > 來源** 中加入外掛目錄。只有官方外掛目錄可以發佈合作夥伴金鑰。

外掛目錄依清單順序合併，外掛從第一個列出它的目錄安裝。官方外掛目錄無法移除，預設排在最前面，也可以調整位置：排在它前面的鏡像會代替它提供官方外掛。這只改變外掛套件的下載位置，不改變外掛套件本身：`com.nginxui.*` 中的 ID 只會從官方金鑰簽署的外掛套件安裝，當這類外掛由其他目錄提供時，外掛市集會標出來源。
