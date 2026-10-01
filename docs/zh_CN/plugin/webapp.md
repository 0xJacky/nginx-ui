---
outline: [2, 3]
---

# 浏览器包

插件可以用 JavaScript 包向 Nginx UI 网页界面添加路由、面板、表单字段和列。浏览器包运行在 Nginx UI 页面内，使用页面自己的 Vue、Vue Router、Pinia 和 antdv-next，因此它的组件外观和行为与界面的其他部分一致。

只需要少量独立页面的插件可以改用[静态页面](./pages.md)，无需构建步骤。

## 声明浏览器包 {#declaring-the-bundle}

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

| 字段 | 含义 |
| --- | --- |
| `bundle_path` | 浏览器包，一个 JavaScript 文件。 |
| `style_path` | 它的样式表，一个 CSS 文件。 |
| `shared` | 构建浏览器包时所用共享库的版本范围。 |
| `chunks` | 浏览器包按需加载的附加文件。参见[分块](#chunks)。 |
| `pages` | [静态页面](./pages.md)。 |

## 构建 {#building}

[@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) 的 Vite 预设生成的浏览器包满足下面的所有规则，还会为你生成 `webapp` 块：

```ts
// webapp/vite.config.ts
import { defineNginxUiPluginConfig } from '@nginxui/plugin-sdk/vite'

export default defineNginxUiPluginConfig({ id: 'io.github.example.myplugin' })
```

`vite build` 会生成 `dist/main.js`、`dist/style.css` 和 `dist/manifest.webapp.json`，后者就是 `webapp` 块，其中的版本范围根据实际安装的版本计算。构建插件包时把它合并进 `plugin.json` 即可。

使用其他构建工具时，请遵守以下规则：

- **一个 IIFE 文件。** 入口是一个立即执行的脚本，不进行代码分割，也不使用 ES 模块导入。之后加载的代码以[分块](#chunks)的形式提供。
- **不包含共享库的副本。** 不要打包 Vue、Vue Router、Pinia、antdv-next、其图标库或 VueUse，也绝不要调用 `createApp`：响应式只在同一个 Vue 实例内有效。把它们标记为外部依赖，并映射到 `window.NginxUI.shared` 上的全局变量：

  | 包 | 全局变量 |
  | --- | --- |
  | `vue` | `window.NginxUI.shared.vue` |
  | `vue-router` | `window.NginxUI.shared.vueRouter` |
  | `pinia` | `window.NginxUI.shared.pinia` |
  | `antdv-next` | `window.NginxUI.shared.antdvNext` |
  | `@antdv-next/icons` | `window.NginxUI.shared.antdvIcons` |
  | `@vueuse/core` | `window.NginxUI.shared.vueuse` |

- **一个纯 CSS 样式表。** 所有样式放在一个 CSS 文件中。编写纯 CSS（Nginx UI 的原子化 CSS 不可用），颜色使用 `--ant-*` 自定义属性，使暗色模式跟随 Nginx UI 的主题，并为每个选择器限定作用域：使用 Vue 的 `scoped` 样式，或者在类名前加上插件 ID。

## 注册插件 {#registering-the-plugin}

浏览器包在脚本执行期间用清单 ID 调用一次 `registerPlugin`：

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

没有调用 `registerPlugin`，或者用其他 ID 调用的浏览器包视为加载失败。无论如何，其他插件都会继续正常工作。

::: tip 提示
页面打开期间插件被禁用、卸载或更新时，Nginx UI 会在不刷新页面的情况下移除它的路由、插槽和设置面板，移除前先调用 `teardown()`。再次启用时，由于同一个脚本不能在一个页面中运行两次，Nginx UI 会对同一个定义再次调用 `setup`。因此 `setup` 可能运行多次，请在其中重置模块级状态。
:::

## Registry {#registry}

`setup(registry)` 收到的对象包含：

| 成员 | 用途 |
| --- | --- |
| `registerRoute(route, opts?)` | 在 Nginx UI 主布局下添加路由。`opts.parent` 把它嵌套到已有的侧边栏条目下，`opts.order` 控制排序。 |
| `registerSlot(slot, component, opts?)` | 把组件挂载到界面的扩展点。参见[插槽](./slots.md)。 |
| `registerTranslations(locale, messages)` | 向某种语言的翻译添加 `{ "<英文文本>": "<翻译>" }`。 |
| `registerSettingsPanel(component)` | 替换自动生成的设置表单。参见[设置面板](#settings-panel)。 |
| `http` | 访问插件自己的 [HTTP 接口](./http.md)的 HTTP 客户端，基础路径为 `./api/plugins/<插件 ID>/http`。返回完整的响应。 |
| `wsUrl(path)` | 连接到插件 HTTP 接口下 `path` 的 WebSocket 地址，包含 Nginx UI 接受连接所需的信息。可选。 |
| `loadChunk(name)` | 加载已声明的[分块](#chunks)。可选。 |
| `coreHttp` | Nginx UI REST API 的客户端，需要 `core_api` 权限。返回响应体。 |
| `manifest` | 插件的清单。 |
| `host` | 只读的 `{ theme, locale, nodeId, username }`。 |

界面中的文字使用 Nginx UI 的 gettext：编写英文原文，并通过 `registerTranslations` 提供翻译。同样的翻译也用于[插槽](./slots.md)的标签、DNS-01 凭据表单和处理指示器。

### WebSocket {#websockets}

浏览器无法为 WebSocket 发送请求头，所以 Nginx UI 把所需信息放在地址中。请始终从 registry 获取这个地址：

```ts
const socket = registry.wsUrl ? new WebSocket(registry.wsUrl('/events')) : undefined
```

在通过 HTTPS 提供的页面上，这个地址使用 `wss:`，会连接到用户选择的节点，并带有 Nginx UI 在请求到达插件之前就会移除的凭据。绝不要自己拼接这样的地址。在没有 `wsUrl` 的版本上，请把实时更新视为不可用。

### 设置面板 {#settings-panel}

`registerSettingsPanel(component)` 用你自己的组件替换根据 `settings_schema` 生成的表单。组件会收到当前值 `settings`，以及 `save(settings)`，后者保存新值并把它们发给进程。

### 宿主对话框 {#host-dialogs}

`window.NginxUI.shared.ui` 提供了浏览器包可以复用的 Nginx UI 对话框。`openDnsCredentialEditor()` 打开 DNS 凭据编辑器，返回新建的凭据，用户取消时返回 `undefined`。调用前请先检查该成员是否存在。

## 分块 {#chunks}

较重的代码（例如带图表的仪表盘）可以放在分块中，由浏览器包在页面首次需要时加载。每个分块都是单独的 IIFE 文件，构建方式与入口相同，并在脚本执行时交出它的导出内容：

```js
window.NginxUI.registerChunk('io.github.example.myplugin', 'dashboard', { Dashboard })
```

入口按名称加载分块，例如作为异步路由组件：

```ts
registry.registerRoute({
  path: 'myplugin/dashboard',
  component: defineAsyncComponent(async () => {
    const { Dashboard } = await registry.loadChunk!('dashboard') as { Dashboard: Component }
    return Dashboard
  }),
})
```

- 每个分块都要在 `webapp.chunks` 中声明：名称匹配 `^[a-z0-9][a-z0-9_-]{0,31}$`，路径是包内的 `.js` 文件，且不能是入口文件，也不能与其他分块共用。`chunks` 需要同时声明 `bundle_path`。
- `loadChunk` 在每个页面中只加载一次每个分块，之后的调用得到相同的导出内容。未声明的名称、加载失败的文件，或者没有用插件 ID 和该名称调用 `registerChunk` 的脚本，都会使 Promise 被拒绝。
- 分块没有自己的样式表：把它的样式放进唯一的样式表，或者让分块自己添加一个限定作用域的 `<style>` 元素。

在较旧的 Nginx UI 版本中，`loadChunk` 可能不存在。浏览器包需要在这些版本上工作时，请先检测它是否存在。

## 共享库版本 {#shared-library-versions}

加载浏览器包之前，Nginx UI 会用自己运行的版本（`window.NginxUI.shared.versions`）检查 `webapp.shared` 中的每个版本范围。如果浏览器包声明了 Nginx UI 不共享的库，或者当前版本不满足声明的范围，它就会被跳过，并在浏览器控制台中给出警告，界面的其余部分继续正常工作。请在浏览器包允许的前提下尽量放宽版本范围。

## 开发 {#developing}

无需打包即可从本地开发服务器加载浏览器包，参见[开发与调试](./development.md#developing-a-browser-bundle)。
