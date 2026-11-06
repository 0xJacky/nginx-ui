---
outline: [2, 3]
---

# Static Pages

A plugin can show plain HTML pages without writing a bundle. Each page gets a
route in Nginx UI and is shown in a frame on the same origin.

```json [plugin.json]
"webapp": {
  "pages": [
    { "path": "status", "file": "pages/status.html", "title": { "en": "MyDNS Status", "zh_CN": "MyDNS 状态" } }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `path` | Route of the page, below `plugins/<plugin id>/pages/`. Not empty. |
| `file` | The HTML file in the package. |
| `title` | Title of the page by language. Include at least `en`. |

Pages and a bundle can be combined in one plugin.

## Talking to Nginx UI

A page asks Nginx UI for what it needs with `postMessage`, always targeted at
its own origin:

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

| Message | Answer |
| --- | --- |
| `nginx-ui:token` | `{ type: 'nginx-ui:token', token }`, the current authentication token for calling the Nginx UI API or the plugin's HTTP endpoints. |
| `nginx-ui:theme` | `{ type: 'nginx-ui:theme', theme }`, `light` or `dark`. |

Nginx UI also sends the theme message on its own whenever the theme changes.
Do not assume an answer arrives at once or only once.

The [@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) package
has a small helper for these messages.
