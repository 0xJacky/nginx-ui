# Browser plugin runtime

This directory is the host side of the Nginx UI plugin system. A plugin ships a
JavaScript bundle; the host loads it after login, hands it a registry, and the
bundle contributes routes, slot components, translations and a settings panel.

```
shared.ts    publishes window.NginxUI (shared modules + registerPlugin)
loader.ts    fetches GET /plugins/webapp, injects bundles, calls setup()
registry.ts  builds the object a bundle receives in setup()
store.ts     everything bundles contributed (routes, slots, settings panels)
semver.ts    range matcher for the shared runtime compatibility check
types.ts     public types, also the source of the SDK type definitions
```

## window.NginxUI

`installSharedRuntime()` runs in `main.ts` before the app is mounted, so the
global exists before any bundle executes.

```ts
window.NginxUI = {
  version,          // host version, from src/version.json
  shared: {
    vue, vueRouter, pinia, antdvNext, antdvIcons, vueuse,
    gettext,        // the host vue3-gettext instance
    http,           // the host API client, baseURL ./api
    versions,       // { vue, 'vue-router', pinia, 'antdv-next', '@vueuse/core' }
    ui: {
      // Opens the host DNS credential form in a modal. Resolves with
      // { id, name, code, provider?, provider_code? } or undefined on cancel.
      // Older hosts lack `ui`, so check for it before calling.
      openDnsCredentialEditor,
    },
  },
  registerPlugin,   // (id, { setup, teardown? }) => void
}
```

`versions` comes from the `__NGINX_UI_SHARED_VERSIONS__` define, which
`vite.config.ts` computes from the dependency ranges in `app/package.json`
with the `^` / `~` prefix stripped. The same values are reported by
`GET /plugins/spec`.

A bundle registers itself synchronously while its `<script>` executes:

```js
window.NginxUI.registerPlugin('com.example.mydns', {
  setup(registry) { /* ... */ },
  teardown() { /* optional */ },
})
```

The loader takes the definition out of the pending map right after the script
finishes loading. A bundle that never calls `registerPlugin` with its own id is
marked `failed` and the other plugins keep loading.

## Registry API

`setup(registry)` receives:

| Member | Description |
| --- | --- |
| `registerRoute(route, { parent?, order? })` | Adds a child route under the `Home` layout with `router.addRoute('Home', route)` and records it in the store. `meta.pluginId` is set automatically, and a route without `meta.name` falls back to the plugin name. `parent` names an existing top level sidebar entry (for example `System`) to nest the item under; `order` sorts it inside that group. |
| `registerSlot(slot, component, { order?, when? })` | Mounts a component into a host slot. `order` ascending, `when(ctx)` may skip the component for a given context. |
| `registerTranslations(locale, messages)` | Merges messages into the host gettext translations reactively. Keys are the English source strings. |
| `registerSettingsPanel(component)` | Replaces the schema-driven form on the plugin settings drawer. The component receives `{ settings, save }`. |
| `http` | Axios instance with `baseURL: ./api/plugins/{id}/http`, carrying the same `Authorization`, `X-Node-ID` and `X-Secure-Session-ID` headers as the core client. Plain axios semantics: it resolves with an `AxiosResponse`. |
| `coreHttp` | The host API client (`./api`). Requires the `core_api` permission. |
| `manifest` | The plugin manifest. |
| `host` | Read-only reactive `{ theme, locale, nodeId, username }`. Stores are never exposed. |

## Slots

The host renders a slot with `<PluginSlot name="..." :context="..." />`. Each
registered component is wrapped in its own `Suspense` and error boundary, so a
throwing plugin component degrades to an inline alert instead of breaking the
page. The default slot of `PluginSlot` is the fallback shown when nothing is
registered.

| Slot | Where | Context |
| --- | --- | --- |
| `certificate.challenge.form:{method}` | Configuration area of the selected challenge method in the certificate form | `{ options }` |
| `dns.credential.form:{provider_code}` | Generated credential fields of the DNS credential form | `{ credential, provider }` |
| `dns.credential.hint:{provider_code}` | Above the credential fields | `{ credential, provider }` |
| `certificate.issue.footer` | Bottom of the certificate issue form | `{ options }` |
| `plugin.settings:{plugin_id}` | Plugin settings drawer | `{ settings }` |
| `sidebar.footer` | Bottom of the sidebar | none |

## Bundle contract

- Build an **IIFE**, one file, no code splitting.
- Never call `createApp`, never bundle a second copy of Vue. Externalize
  `vue`, `vue-router`, `pinia`, `antdv-next`, `@antdv-next/icons` and
  `@vueuse/core` onto `window.NginxUI.shared.*`.
- Ship styles as a separate `style.css`. UnoCSS is not shared, so write plain
  CSS and use the `--ant-*` variables for colours so dark mode follows the
  host. Scope selectors or prefix them with the plugin id; do not touch global
  styles.
- Declare the ranges the bundle was built against in the manifest:

```jsonc
"webapp": {
  "bundle_path": "webapp/dist/main.js",
  "style_path": "webapp/dist/style.css",
  "shared": { "vue": ">=3.5.42 <4", "antdv-next": "~1.5" }
}
```

  The loader checks every entry against `window.NginxUI.shared.versions` before
  injecting anything. A bundle that does not match is marked `incompatible`,
  logs a warning and is skipped; the server side of the plugin keeps working.

- Keep user-facing text in English in the source and translate it through
  `registerTranslations`.

## Zero-build iframe pages

A manifest may declare pages instead of a bundle:

```jsonc
"webapp": {
  "pages": [
    { "path": "status", "title": { "en": "Status", "zh_CN": "状态" }, "file": "pages/index.html" }
  ]
}
```

The loader registers one route per page at `plugins/{id}/pages/{path}` rendering
`views/plugin/IframePage.vue`, which embeds a same-origin iframe pointing at the
static file. The page talks to the host over `postMessage`:

```js
parent.postMessage({ type: 'nginx-ui:token' }, location.origin)
parent.postMessage({ type: 'nginx-ui:theme' }, location.origin)
// replies: { type: 'nginx-ui:token', token } / { type: 'nginx-ui:theme', theme: 'light' | 'dark' }
```

The host answers only messages whose source is that iframe, and pushes a new
`nginx-ui:theme` message whenever the host theme changes.

## Developing a plugin

Run `vite build --watch` in the plugin repository so the bundle is rebuilt on
every change, serve the build output with any static server, then open
**System > Plugins > Dev plugin URL** and paste the address of the plugin's
`plugin.json`. The address is persisted in the browser, and the loader loads
that plugin in addition to the installed ones, resolving `bundle_path` and
`style_path` relative to the manifest URL. There is no HMR: reload the page
after a rebuild. Clear the field to stop loading it.
