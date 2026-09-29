# Browser plugin runtime

This directory is the host side of the Nginx UI plugin system. A plugin ships a
JavaScript bundle; the host loads it after login, hands it a registry, and the
bundle contributes routes, slot components, translations and a settings panel.

```
shared.ts    publishes window.NginxUI (shared modules + registerPlugin + registerChunk)
loader.ts    fetches GET /plugins/webapp, injects bundles, calls setup()
chunks.ts    on-demand chunk loading and the queue every plugin script runs through
slots.ts     helpers that list slot registrations, also by name prefix
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
  registerChunk,    // (pluginId, name, exports) => void, called by chunk files
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
| `registerSlot(slot, component, { order?, when?, label?, sortValue?, filters? })` | Mounts a component into a host slot. `order` ascending, `when(ctx)` may skip the component for a given context. `label`, `sortValue` and `filters` are read only by the `nginx_log.view:{key}` and `nginx_log.list.column:{key}` slots, see below. |
| `registerTranslations(locale, messages)` | Merges messages into the host gettext translations reactively. Keys are the English source strings. |
| `registerSettingsPanel(component)` | Replaces the schema-driven form on the plugin settings drawer. The component receives `{ settings, save }`. |
| `loadChunk(name)` | Loads an on-demand chunk declared in `webapp.chunks` and resolves with the exports the chunk handed to `registerChunk`. See Chunks. |
| `http` | Axios instance with `baseURL: ./api/plugins/{id}/http`, carrying the same `Authorization`, `X-Node-ID` and `X-Secure-Session-ID` headers as the core client. Plain axios semantics: it resolves with an `AxiosResponse`. |
| `wsUrl(path)` | Absolute `ws:` or `wss:` URL of a path under the plugin `http` route, built like the host's own WebSocket URLs: the session token and the selected node id travel in the query string, because a browser WebSocket cannot send headers. The plugin route accepts those query credentials for upgrade requests only, and strips them before the request reaches the plugin. A plugin must not build such a URL itself. |
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
| `nginx_log.view:{key}` | A view mode of the log page, listed after the raw view with the registration `label` and selected with `?view={key}`. The key `raw` is ignored. Without a `view` in the link, access logs open the view with the key `structured` when a plugin registers one, error logs open the raw view. With no plugin view the page shows the raw view only and no switch. `when(ctx)` decides whether the mode is offered for a file. The page draws the log file picker and the view switch in a header row above the view, so a view does not render them itself. | `{ path, type }` |
| `nginx_log.list.toolbar` | Actions area above the log list | `{ type }` |
| `nginx_log.list.column:{key}` | One extra column of the log list, titled with `label`, placed after the host columns and before the actions, ordered by `order`. `when(ctx)` is called once per list with `{ type }` and decides whether the column exists at all, header and cells. | `{ row }` |
| `nginx_log.list.row.actions` | Per row actions of the log list | `{ row }` |
| `site.log.actions` | Log actions of one site, in the site editor and in the site list. A path is the site's own log directive, else the nginx default log the site falls back to, else an empty string; the matching `...Inherited` flag is true for the fallback. The site list reads the paths from the row, and the editor asks for the default logs only when a plugin registered this slot. | `{ accessLogPath, accessLogInherited, errorLogPath, errorLogInherited, siteName }` |

### Log list columns

A column registration may add options the host applies in the browser, because
the log list arrives whole:

```ts
registry.registerSlot('nginx_log.list.column:status', StatusCell, {
  label: 'Index Status',                      // English source string, translated by the host
  order: 10,
  sortValue: row => statusRank(row.path),     // numbers by magnitude, strings by locale order, null and undefined last
  filters: [{ label: 'Indexed', value: 'indexed', match: row => isIndexed(row.path) }],
})
```

Several selected filters of one column combine with OR, columns combine with
AND. `row` always has `path`, `type`, `name` and `config_file`, and may carry
more fields, which a plugin must ignore. Sorting and filtering are evaluated
each time the list is fetched: on load and after a change of sort, filter or
search.

A column that only suits one kind of log decides in `when`, which receives the
list context, for example `when: ctx => ctx.type === 'access'`. The component
of a cell only gets `{ row }`.

Slot labels go through the host gettext, so a plugin supplies translations with
`registerTranslations` under the same English string.

## Processing indicator

`host.activity.set` entries of a plugin appear in the processing indicator of
the header next to the host tasks. The server sends them as
`plugins: [{ plugin_id, key, label }]` in the `processing_status` event, and
the indicator shows `$gettext(label)`. The translations come from the plugin
through `registerTranslations`, and the English label is shown when there is
none.

## Chunks

A bundle may declare more IIFE files in `webapp.chunks` and load them when a
page needs them:

```jsonc
"webapp": {
  "bundle_path": "webapp/dist/main.js",
  "chunks": { "search": "webapp/dist/search.js", "dashboard": "webapp/dist/dashboard.js" }
}
```

A chunk hands its exports over while its script executes, and the entry reads
them with `registry.loadChunk`:

```js
// in the chunk
window.NginxUI.registerChunk('com.example.logs', 'dashboard', { Dashboard })
// in the entry
const { Dashboard } = await registry.loadChunk('dashboard')
```

`GET /plugins/webapp` lists the address of each chunk, and the static route
serves the files under the directory of the bundle, so a chunk lives next to
it. `loadChunk`:

- rejects a name that is not declared, without any request;
- loads a chunk once per page, concurrent and later calls share one promise;
- appends `?v={version}` like the entry does;
- runs one script at a time. Entry bundles and chunks go through one queue,
  because each hands its result over on a global that is read right after the
  script ran;
- rejects when the file fails to load or the script did not call `registerChunk`
  with the plugin id and the requested name, and a later call may try again.

A chunk has no stylesheet reference of its own, so its styles go into the
plugin `style.css` or the chunk injects a `<style>` element itself.

## Bundle contract

- Build an **IIFE**, one entry file, no code splitting. Code loaded later is
  declared as chunks, each another IIFE file.
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
