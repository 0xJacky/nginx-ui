---
outline: [2, 3]
---

# 瀏覽器套件

外掛可以用 JavaScript 套件向 Nginx UI 網頁介面加入路由、面板、表單欄位和欄。瀏覽器套件在 Nginx UI 頁面內執行，使用頁面自己的 Vue、Vue Router、Pinia 和 antdv-next，因此它的元件外觀和行為與介面的其他部分一致。

只需要少量獨立頁面的外掛可以改用[靜態頁面](./pages.md)，不需建置步驟。

## 宣告瀏覽器套件 {#declaring-the-bundle}

```json [plugin.json]
"webapp": {
  "bundle_path": "webapp/dist/main.js",
  "style_path": "webapp/dist/style.css",
  "shared": {
    "vue": ">=3.5.42 <4",
    "antdv-next": "~1.5"
  },
  "chunks": {
    "dashboard": "webapp/dist/chunks/dashboard.js"
  }
}
```

| 欄位 | 意義 |
| --- | --- |
| `bundle_path` | 瀏覽器套件，一個 JavaScript 檔案。 |
| `style_path` | 它的樣式表，一個 CSS 檔案。 |
| `shared` | 建置瀏覽器套件時所用共用函式庫的版本範圍。 |
| `chunks` | 瀏覽器套件依需要載入的附加檔案。參見[分塊](#chunks)。 |
| `pages` | [靜態頁面](./pages.md)。 |

## 建置 {#building}

[@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) 的 Vite 預設設定產生的瀏覽器套件符合下列所有規則，還會為你產生 `webapp` 區塊：

```ts
// webapp/vite.config.ts
import { defineNginxUiPluginConfig } from '@nginxui/plugin-sdk/vite'

export default defineNginxUiPluginConfig({ id: 'io.github.example.myplugin' })
```

`vite build` 會產生 `dist/main.js`、`dist/style.css` 和 `dist/manifest.webapp.json`，後者就是 `webapp` 區塊，其中的版本範圍根據實際安裝的版本計算。建置外掛套件時把它合併進 `plugin.json` 即可。

使用其他建置工具時，請遵守以下規則：

- **一個 IIFE 檔案。** 入口是一個立即執行的指令碼，不進行程式碼分割，也不使用 ES 模組匯入。之後載入的程式碼以[分塊](#chunks)的形式提供。
- **不包含共用函式庫的副本。** 不要打包 Vue、Vue Router、Pinia、antdv-next、其圖示庫或 VueUse，也絕不要呼叫 `createApp`：響應式只在同一個 Vue 實例內有效。把它們標記為外部相依，並對應到 `window.NginxUI.shared` 上的全域變數：

  | 套件 | 全域變數 |
  | --- | --- |
  | `vue` | `window.NginxUI.shared.vue` |
  | `vue-router` | `window.NginxUI.shared.vueRouter` |
  | `pinia` | `window.NginxUI.shared.pinia` |
  | `antdv-next` | `window.NginxUI.shared.antdvNext` |
  | `@antdv-next/icons` | `window.NginxUI.shared.antdvIcons` |
  | `@vueuse/core` | `window.NginxUI.shared.vueuse` |

- **一個純 CSS 樣式表。** 所有樣式放在一個 CSS 檔案中。撰寫純 CSS（Nginx UI 的原子化 CSS 無法使用），顏色使用 `--ant-*` 自訂屬性，使深色模式跟隨 Nginx UI 的佈景主題，並為每個選擇器限定範圍：使用 Vue 的 `scoped` 樣式，或者在類別名稱前加上外掛 ID。

## 註冊外掛 {#registering-the-plugin}

瀏覽器套件在指令碼執行期間用清單 ID 呼叫一次 `registerPlugin`：

```ts
import { defineNginxUIPlugin, registerPlugin } from '@nginxui/plugin-sdk'
import ChallengeForm from './ChallengeForm.vue'

registerPlugin('io.github.example.myplugin', defineNginxUIPlugin({
  setup(registry) {
    registry.registerSlot('certificate.challenge.form:dns01', ChallengeForm)
    registry.registerTranslations('zh_TW', { 'API token': 'API 權杖' })
  },
  teardown() {
    // stop timers and connections started outside components
  },
}))
```

沒有呼叫 `registerPlugin`，或者用其他 ID 呼叫的瀏覽器套件視為載入失敗。無論如何，其他外掛都會繼續正常運作。

::: tip 提示
頁面開啟期間外掛被停用、解除安裝或更新時，Nginx UI 會在不重新整理頁面的情況下移除它的路由、插槽和設定面板，移除前先呼叫 `teardown()`。再次啟用時，由於同一個指令碼不能在一個頁面中執行兩次，Nginx UI 會對同一個定義再次呼叫 `setup`。因此 `setup` 可能執行多次，請在其中重設模組層級的狀態。
:::

## Registry {#registry}

`setup(registry)` 收到的物件包含：

| 成員 | 用途 |
| --- | --- |
| `registerRoute(route, opts?)` | 在 Nginx UI 主版面下加入路由。`opts.parent` 把它巢狀到已有的側邊欄項目下，`opts.order` 控制排序。 |
| `registerSlot(slot, component, opts?)` | 把元件掛載到介面的擴充點。參見[插槽](./slots.md)。 |
| `registerTranslations(locale, messages)` | 向某種語言的翻譯加入 `{ "<英文文字>": "<翻譯>" }`。 |
| `registerSettingsPanel(component)` | 取代自動產生的設定表單。參見[設定面板](#settings-panel)。 |
| `http` | 存取外掛自己的 [HTTP 介面](./http.md)的 HTTP 用戶端，基礎路徑為 `./api/plugins/<外掛 ID>/http`。回傳完整的回應。 |
| `wsUrl(path)` | 連線到外掛 HTTP 介面下 `path` 的 WebSocket 網址，包含 Nginx UI 接受連線所需的資訊。選用。 |
| `loadChunk(name)` | 載入已宣告的[分塊](#chunks)。選用。 |
| `coreHttp` | Nginx UI REST API 的用戶端，需要 `core_api` 權限。回傳回應本文。 |
| `manifest` | 外掛的清單。 |
| `host` | 唯讀的 `{ theme, locale, nodeId, username }`。 |

介面中的文字使用 Nginx UI 的 gettext：撰寫英文原文，並透過 `registerTranslations` 提供翻譯。同樣的翻譯也用於[插槽](./slots.md)的標籤、DNS-01 認證表單和處理指示器。

### WebSocket {#websockets}

瀏覽器無法為 WebSocket 傳送請求標頭，所以 Nginx UI 把所需資訊放在網址中。請一律從 registry 取得這個網址：

```ts
const socket = registry.wsUrl ? new WebSocket(registry.wsUrl('/events')) : undefined
```

在透過 HTTPS 提供的頁面上，這個網址使用 `wss:`，會連線到使用者選擇的節點，並帶有 Nginx UI 在請求到達外掛之前就會移除的認證。絕不要自己組合這樣的網址。在沒有 `wsUrl` 的版本上，請把即時更新視為無法使用。

### 設定面板 {#settings-panel}

`registerSettingsPanel(component)` 用你自己的元件取代根據 `settings_schema` 產生的表單。元件會收到目前值 `settings`，以及 `save(settings)`，後者儲存新值並把它們傳給進程。

### 宿主對話框 {#host-dialogs}

`window.NginxUI.shared.ui` 提供了瀏覽器套件可以重複使用的 Nginx UI 對話框。`openDnsCredentialEditor()` 開啟 DNS 認證編輯器，回傳新建的認證，使用者取消時回傳 `undefined`。呼叫前請先檢查該成員是否存在。

## 分塊 {#chunks}

較重的程式碼（例如帶圖表的儀表板）可以放在分塊中，由瀏覽器套件在頁面首次需要時載入。每個分塊都是單獨的 IIFE 檔案，建置方式與入口相同，並在指令碼執行時交出它的匯出內容：

```js
window.NginxUI.registerChunk('io.github.example.myplugin', 'dashboard', { Dashboard })
```

入口依名稱載入分塊，例如作為非同步路由元件：

```ts
registry.registerRoute({
  path: 'myplugin/dashboard',
  component: defineAsyncComponent(async () => {
    const { Dashboard } = await registry.loadChunk!('dashboard') as { Dashboard: Component }
    return Dashboard
  }),
})
```

- 每個分塊都要在 `webapp.chunks` 中宣告：名稱符合 `^[a-z0-9][a-z0-9_-]{0,31}$`，路徑是套件內的 `.js` 檔案，且不能是入口檔案，也不能與其他分塊共用。`chunks` 需要同時宣告 `bundle_path`。
- `loadChunk` 在每個頁面中只載入一次每個分塊，之後的呼叫得到相同的匯出內容。未宣告的名稱、載入失敗的檔案，或者沒有用外掛 ID 和該名稱呼叫 `registerChunk` 的指令碼，都會使 Promise 被拒絕。
- 分塊沒有自己的樣式表：把它的樣式放進唯一的樣式表，或者讓分塊自己加入一個限定範圍的 `<style>` 元素。

在較舊的 Nginx UI 版本中，`loadChunk` 可能不存在。瀏覽器套件需要在這些版本上運作時，請先偵測它是否存在。

## 共用函式庫版本 {#shared-library-versions}

載入瀏覽器套件之前，Nginx UI 會用自己執行的版本（`window.NginxUI.shared.versions`）檢查 `webapp.shared` 中的每個版本範圍。如果瀏覽器套件宣告了 Nginx UI 不共用的函式庫，或者目前版本不符合宣告的範圍，它就會被略過，並在瀏覽器主控台中給出警告，介面的其餘部分繼續正常運作。請在瀏覽器套件允許的前提下盡量放寬版本範圍。

## 開發 {#developing}

不需打包即可從本機開發伺服器載入瀏覽器套件，參見[開發與除錯](./development.md#developing-a-browser-bundle)。
