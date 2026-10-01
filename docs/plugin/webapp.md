---
outline: [2, 3]
---

# Browser Bundle

A plugin can add routes, panels, form fields and columns to the Nginx UI web
interface with a JavaScript bundle. The bundle runs inside the Nginx UI page
and uses the page's own Vue, Vue Router, Pinia and antdv-next, so its
components look and behave like the rest of the interface.

A plugin that only needs a few pages of its own can use
[static pages](./pages.md) instead, which need no build step.

## Declaring the Bundle

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

| Field | Meaning |
| --- | --- |
| `bundle_path` | The bundle, one JavaScript file. |
| `style_path` | Its stylesheet, one CSS file. |
| `shared` | The version ranges of the shared libraries the bundle was built against. |
| `chunks` | Extra files the bundle loads on demand. See [Chunks](#chunks). |
| `pages` | [Static pages](./pages.md). |

## Building

The [@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) Vite
preset produces a bundle that follows every rule below and writes the
`webapp` block for you:

```ts
// webapp/vite.config.ts
import { defineNginxUiPluginConfig } from '@nginxui/plugin-sdk/vite'

export default defineNginxUiPluginConfig({ id: 'io.github.example.myplugin' })
```

`vite build` then writes `dist/main.js`, `dist/style.css` and
`dist/manifest.webapp.json`, the `webapp` block with version ranges computed
from what is actually installed. Merge it into `plugin.json` when you build
the package.

With another build tool, follow these rules:

- **One IIFE file.** The entry is a single immediately invoked script, with
  no code splitting and no ES module imports. Code loaded later comes as
  [chunks](#chunks).
- **No copy of the shared libraries.** Do not bundle Vue, Vue Router, Pinia,
  antdv-next, its icons or VueUse, and never call `createApp`: reactivity
  only works within one Vue instance. Mark them external and map them to the
  globals on `window.NginxUI.shared`:

  | Package | Global |
  | --- | --- |
  | `vue` | `window.NginxUI.shared.vue` |
  | `vue-router` | `window.NginxUI.shared.vueRouter` |
  | `pinia` | `window.NginxUI.shared.pinia` |
  | `antdv-next` | `window.NginxUI.shared.antdvNext` |
  | `@antdv-next/icons` | `window.NginxUI.shared.antdvIcons` |
  | `@vueuse/core` | `window.NginxUI.shared.vueuse` |

- **One plain stylesheet.** Ship all styles in one CSS file. Write plain CSS
  (the utility CSS of Nginx UI is not available), use the `--ant-*` custom
  properties for colors so dark mode follows the Nginx UI theme, and scope
  every selector, with Vue `scoped` styles or class names prefixed by the
  plugin id.

## Registering the Plugin

While its script runs, the bundle calls `registerPlugin` once, with its
manifest id:

```ts
import { defineNginxUIPlugin, registerPlugin } from '@nginxui/plugin-sdk'
import ChallengeForm from './ChallengeForm.vue'

registerPlugin('io.github.example.myplugin', defineNginxUIPlugin({
  setup(registry) {
    registry.registerSlot('certificate.challenge.form:dns01', ChallengeForm)
    registry.registerTranslations('zh_CN', { 'API token': 'API 令牌' })
  },
  teardown() {
    // stop timers and connections started outside components
  },
}))
```

A bundle that does not call `registerPlugin`, or calls it with another id,
counts as failed to load. Other plugins keep working either way.

::: tip
When a plugin is disabled, uninstalled or updated while the page is open,
Nginx UI removes its routes, slots and settings panel without a reload,
calling `teardown()` first. When it is enabled again, Nginx UI calls `setup`
again on the same definition, since a script cannot run twice on one page.
Expect `setup` to run more than once and reset any module level state in it.
:::

## Registry

`setup(registry)` receives:

| Member | Purpose |
| --- | --- |
| `registerRoute(route, opts?)` | Adds a route under the main layout of Nginx UI. `opts.parent` nests it under an existing sidebar entry, `opts.order` sorts it there. |
| `registerSlot(slot, component, opts?)` | Mounts a component into an extension point of the interface. See [Slots](./slots.md). |
| `registerTranslations(locale, messages)` | Adds `{ "<English text>": "<translation>" }` to the translations of a language. |
| `registerSettingsPanel(component)` | Replaces the generated settings form. See [Settings Panel](#settings-panel). |
| `http` | HTTP client for the plugin's own [HTTP endpoints](./http.md), based at `./api/plugins/<plugin id>/http`. Resolves with the full response. |
| `wsUrl(path)` | The URL of a WebSocket to `path` under the plugin's HTTP endpoints, with what Nginx UI needs to accept it. Optional. |
| `loadChunk(name)` | Loads a declared [chunk](#chunks). Optional. |
| `coreHttp` | Client for the Nginx UI REST API, available with the `core_api` permission. Resolves with the response body. |
| `manifest` | The plugin's manifest. |
| `host` | Read-only `{ theme, locale, nodeId, username }`. |

Text in the interface uses the gettext of Nginx UI: write English source
strings and ship their translations with `registerTranslations`. The same
translations serve the labels of [slots](./slots.md), of the DNS-01
credential form and of the processing indicator.

### WebSockets

A browser cannot send headers with a WebSocket, so Nginx UI puts what it needs
into the URL. Always ask the registry for that URL:

```ts
const socket = registry.wsUrl ? new WebSocket(registry.wsUrl('/events')) : undefined
```

The URL uses `wss:` on pages served over HTTPS, reaches the node the person
selected and carries credentials that Nginx UI removes before the request
reaches the plugin. Never build such a URL yourself. On versions without
`wsUrl`, treat live updates as unavailable.

### Settings Panel

`registerSettingsPanel(component)` replaces the form generated from
`settings_schema` with a component of your own. It receives `settings`, the
current values, and `save(settings)`, which stores new values and sends them to
the process.

### Host Dialogs

`window.NginxUI.shared.ui` exposes dialogs of Nginx UI a bundle can reuse.
`openDnsCredentialEditor()` opens the DNS credential editor and resolves with
the created credential, or `undefined` when the person cancels. Check that a
member exists before calling it.

## Chunks

Heavy code, such as a dashboard with charts, can live in chunks the bundle
loads when a page first needs it. Each chunk is its own IIFE file, built like
the entry, and hands its exports over while its script runs:

```js
window.NginxUI.registerChunk('io.github.example.myplugin', 'dashboard', { Dashboard })
```

The entry loads it by name, for example as an async route component:

```ts
registry.registerRoute({
  path: 'myplugin/dashboard',
  component: defineAsyncComponent(async () => {
    const { Dashboard } = await registry.loadChunk!('dashboard') as { Dashboard: Component }
    return Dashboard
  }),
})
```

- Every chunk is declared in `webapp.chunks`: a name matching
  `^[a-z0-9][a-z0-9_-]{0,31}$` and a `.js` path in the package that is not the
  entry and not used by another chunk. `chunks` requires `bundle_path`.
- `loadChunk` loads each chunk once per page; later calls get the same
  exports. An undeclared name, a file that fails to load or a script that does
  not call `registerChunk` with the plugin id and that name rejects the
  promise.
- A chunk has no stylesheet of its own: put its styles into the one
  stylesheet, or have the chunk add a scoped `<style>` element itself.

`loadChunk` is optional in older versions of Nginx UI. Feature-detect it when
your bundle has to work there.

## Shared Library Versions

Before it loads a bundle, Nginx UI checks every range in `webapp.shared`
against the versions it runs (`window.NginxUI.shared.versions`). A bundle that
names a library Nginx UI does not share, or a range the running version does
not satisfy, is skipped with a warning in the browser console, and the rest of
the interface keeps working. Keep the ranges as wide as your bundle allows.

## Developing

Load a bundle from a local development server without packaging it, see
[Develop and Debug](./development.md#developing-a-browser-bundle).
