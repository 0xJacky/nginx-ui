---
outline: [2, 3]
---

# 静态页面

插件无需编写浏览器包，也可以显示普通的 HTML 页面。每个页面在 Nginx UI 中都有一个路由，并在同源的框架中显示。

```json [plugin.json]
"webapp": {
  "pages": [
    { "path": "status", "file": "pages/status.html", "title": { "en": "MyDNS Status", "zh_CN": "MyDNS 状态" } }
  ]
}
```

| 字段 | 含义 |
| --- | --- |
| `path` | 页面的路由，位于 `plugins/<插件 ID>/pages/` 之下。不能为空。 |
| `file` | 插件包中的 HTML 文件。 |
| `title` | 各语言的页面标题，至少包含 `en`。 |

一个插件可以同时使用页面和浏览器包。

## 与 Nginx UI 通信 {#talking-to-nginx-ui}

页面通过 `postMessage` 向 Nginx UI 请求所需内容，目标始终是自己的源：

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

| 消息 | 回复 |
| --- | --- |
| `nginx-ui:token` | `{ type: 'nginx-ui:token', token }`，当前的身份验证令牌，用于调用 Nginx UI API 或插件的 HTTP 接口。 |
| `nginx-ui:theme` | `{ type: 'nginx-ui:theme', theme }`，值为 `light` 或 `dark`。 |

主题变化时，Nginx UI 也会主动发送主题消息。不要假定回复会立即到达，或只到达一次。

[@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) 包为这些消息提供了一个小型辅助工具。
