---
outline: [2, 3]
---

# 靜態頁面

外掛不需撰寫瀏覽器套件，也可以顯示一般的 HTML 頁面。每個頁面在 Nginx UI 中都有一個路由，並在同源的框架中顯示。

```json [plugin.json]
"webapp": {
  "pages": [
    { "path": "status", "file": "pages/status.html", "title": { "en": "MyDNS Status", "zh_TW": "MyDNS 狀態" } }
  ]
}
```

| 欄位 | 意義 |
| --- | --- |
| `path` | 頁面的路由，位於 `plugins/<外掛 ID>/pages/` 之下。不能為空。 |
| `file` | 外掛套件中的 HTML 檔案。 |
| `title` | 各語言的頁面標題，至少包含 `en`。 |

一個外掛可以同時使用頁面和瀏覽器套件。

## 與 Nginx UI 通訊 {#talking-to-nginx-ui}

頁面透過 `postMessage` 向 Nginx UI 請求所需內容，目標一律是自己的來源：

```js
parent.postMessage({ type: 'nginx-ui:token' }, location.origin)
parent.postMessage({ type: 'nginx-ui:theme' }, location.origin)

window.addEventListener('message', (event) => {
  if (event.origin !== location.origin)
    return
  if (event.data?.type === 'nginx-ui:token')
    token = event.data.token
  if (event.data?.type === 'nginx-ui:theme')
    document.documentElement.dataset.theme = event.data.theme
})
```

| 訊息 | 回覆 |
| --- | --- |
| `nginx-ui:token` | `{ type: 'nginx-ui:token', token }`，目前的身分驗證權杖，用於呼叫 Nginx UI API 或外掛的 HTTP 介面。 |
| `nginx-ui:theme` | `{ type: 'nginx-ui:theme', theme }`，值為 `light` 或 `dark`。 |

佈景主題變化時，Nginx UI 也會主動傳送佈景主題訊息。不要假定回覆會立即到達，或只到達一次。

[@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) 套件為這些訊息提供了一個小型輔助工具。
