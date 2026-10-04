---
outline: [2, 3]
---

# 清单文件

每个插件的插件包根目录都有一个 `plugin.json`。它是一个 UTF-8 编码的 JSON 对象，说明插件是什么、包含什么、请求什么。Nginx UI 在安装插件前会校验它，违反本页任何规则的插件包都会被拒绝。

清单的 [JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/plugin.schema.json) 可以为编辑器提供补全和校验：

```json
{
  "$schema": "https://raw.githubusercontent.com/nginxui/plugin-spec/main/schema/plugin.schema.json",
  "id": "io.github.example.mydns"
}
```

## 字段 {#fields}

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `id` | string | 是 | 唯一的插件 ID。参见[标识](#identity)。 |
| `name` | string | 是 | 显示名称。 |
| `version` | string | 是 | 插件自身的版本，使用[语义化版本](https://semver.org/lang/zh-CN/)。 |
| `description` | string | 否 | 插件列表中显示的一行简介。 |
| `i18n` | object | 否 | `name`、`description`、`permission_reasons` 和截图说明的翻译。参见[翻译](#translations)。 |
| `homepage_url` | string | 否 | 文档或仓库链接。 |
| `icon_path` | string | 否 | 插件包内图标文件的路径。 |
| `api_version` | integer | 是 | 插件使用的协议代次，目前固定为 `1`。 |
| `min_nginx_ui_version` | string | 否 | 插件支持的最低 Nginx UI 版本。参见[版本与兼容性](./versioning.md)。 |
| `server` | object | 否\* | 服务端进程。参见[服务端进程](#server-process)。 |
| `webapp` | object | 否\* | 浏览器包或静态页面。参见[浏览器包](./webapp.md)。 |
| `content` | object | 否\* | 模板和翻译文件。参见[模板与翻译](./capabilities/content.md)。 |
| `capabilities` | string[] | 否 | 插件实现的能力。参见[能力](#capabilities)。 |
| `permissions` | string[] | 否 | 插件需要的 Nginx UI 功能。参见[权限与安全](./permissions.md)。 |
| `requires` | object[] | 否 | 插件依赖的其他插件。参见[依赖与冲突](#dependencies-and-conflicts)。 |
| `requires_capabilities` | string[] | 否 | 需要其他已启用插件提供的能力。 |
| `conflicts` | string[] | 否 | 绝不能与本插件同时运行的插件。 |
| `events` | string[] | 否 | 插件订阅的事件。参见[宿主 API](./host-api.md#events)。 |
| `cron` | object[] | 否 | 定时调用。参见[宿主 API](./host-api.md#scheduled-tasks)。 |
| `network_hosts` | string[] | 否 | 插件打算访问的主机，会展示给安装插件的人。 |
| `permission_reasons` | object | 否 | 插件请求每项权限的原因。参见[说明权限用途](./permissions.md#explaining-permissions)。 |
| `screenshots` | object[] | 否 | 插件目录展示用的使用截图。参见[截图](#screenshots)。 |
| `settings_schema` | object | 否 | 设置表单。参见[设置](#settings)。 |
| `dns01`、`http`、`notify`、`probe`、`mcp`、`storage`、`deploy`、`blocklist`、`discovery`、`log_sink` | object | 声明对应能力时 | 各能力的配置。 |

\* 清单至少声明 `server`、`webapp` 和 `content` 中的一个，否则插件什么也不会安装。

::: info 说明
清单中的所有路径（`icon_path`、`server.executables`、`webapp` 中的路径、`content` 中的路径）都相对于插件包根目录，并且必须是[安全的相对路径](./packaging.md#safe-paths)。截图路径是唯一的例外，它相对于仓库，参见[截图](#screenshots)。
:::

## 标识 {#identity}

- `id` 由小写字母、数字和连字符组成，至少两段，以点分隔，最长 64 个字符：`^[a-z0-9]+(\.[a-z0-9-]+)+$`，例如 `io.github.example.mydns`。插件的 ID 在整个生命周期内都不会改变。[命名规则](./naming.md#plugin-ids)说明了如何选择 ID。
- `name` 必填且不能为空。
- `version` 遵循语义化版本 2.0.0，例如 `1.4.0` 或 `2.0.0-beta.1`。预发布版本会让该版本进入较不稳定的[发布渠道](./catalog.md#release-channels)。
- `api_version` 为 `1`。Nginx UI 会拒绝其他任何值。

## 翻译 {#translations}

`i18n` 把名称和描述翻译成 Nginx UI 界面的语言：

```json [plugin.json]
"i18n": {
  "zh_CN": { "name": "DNS-01 验证", "description": "使用任意 DNS 服务商完成 ACME DNS-01 验证。" },
  "ja_JP": { "name": "DNS-01 チャレンジ" }
}
```

只需翻译你想支持的语言。顶层的 `name` 和 `description` 是英文文本，没有翻译的语言会显示它们。[`permission_reasons`](./permissions.md#explaining-permissions) 也可以用同样的方式按语言翻译。

键使用 Nginx UI 界面的语言代码，例如 `zh_CN`、`zh_TW` 或 `ja_JP`。Nginx UI 显示它界面已有的语言，其他语言显示英文，所以插件可以先于部分 Nginx UI 提供某种语言。[`nginx-ui plugin lint`](./rules.md#manifest-i18n) 会报告不是语言代码的键，并对运行它的 Nginx UI 没有的语言发出警告。

## 截图 {#screenshots}

`screenshots` 最多列出 8 张插件使用中的截图，按顺序显示在插件目录的展示页上：

```json [plugin.json]
"screenshots": [
  {
    "id": "dashboard",
    "path": "docs/screenshots/dashboard.png",
    "dark_path": "docs/screenshots/dashboard-dark.png",
    "caption": "Traffic at a glance"
  }
],
"i18n": {
  "zh_CN": { "screenshot_captions": { "dashboard": "一览流量" } }
}
```

和清单中的其他路径不同，`path` 和 `dark_path` 相对于插件仓库的根目录，而不是插件包，这样图片不会打进插件包。插件目录在每个版本的 tag 上读取它们，所以发布新版本就能更新截图。请使用 PNG、JPEG 或 WebP 图片，宽高比约 16:10，宽度 1280 到 1920 像素。`dark_path` 是同一画面的深色主题版本，Nginx UI 界面为深色时显示它。`id` 是截图的名称，由小写字母、数字和短横线组成。`caption` 使用英文，`i18n` 中的 `screenshot_captions` 按 `id` 翻译它。[`nginx-ui plugin lint`](./rules.md#manifest-screenshots) 会检查路径和说明。

## 服务端进程 {#server-process}

```json [plugin.json]
"server": {
  "executables": {
    "linux-amd64": "dist/linux-amd64/mydns",
    "linux-arm64": "dist/linux-arm64/mydns",
    "windows-amd64": "dist/windows-amd64/mydns.exe"
  },
  "lifecycle": "on_demand",
  "idle_timeout_seconds": 300,
  "resources": { "memory_mb": 256, "cpu_percent": 50, "recommended_memory_mb": 1024 }
}
```

| 字段 | 含义 |
| --- | --- |
| `executables` | 平台键（`<goos>-<goarch>`，参见[命名规则](./naming.md#platform-keys)）到该平台可执行文件的映射。 |
| `command` | 解释型插件的命令行，例如 `["python3", "server/main.py"]`。当 `executables` 没有当前平台的条目时使用。 |
| `lifecycle` | `resident`（默认）表示插件启用期间进程持续运行；`on_demand` 表示需要时才启动。 |
| `idle_timeout_seconds` | 用于 `on_demand`：进程空闲多久后被 Nginx UI 停止。不能为负数。 |
| `resources` | 进程需要的资源。参见[资源](#resources)。 |

`server` 需要 `executables` 或 `command`。`command` 的第一个元素要么是在 `PATH` 中查找的程序（例如 `python3`），要么是插件包内的路径。Nginx UI 安装插件包时会为每个声明的可执行文件加上可执行权限；如果当前平台既没有可执行文件也没有命令，插件就无法启动。

[生命周期](./lifecycle.md)一页介绍了进程如何启动、检查和停止。

### 资源 {#resources}

`server.resources` 告诉 Nginx UI 和安装插件的人进程需要多少资源。三个值都是可选的，且不能为负数。

| 字段 | 含义 |
| --- | --- |
| `memory_mb` | 进程最多使用的内存，单位 MiB。 |
| `cpu_percent` | 进程最多使用的 CPU 时间，以单个核心的百分比表示：`100` 为一个核心，`250` 为两个半核心。 |
| `recommended_memory_mb` | 机器（或运行 Nginx UI 的容器）应有的内存，包括 Nginx UI 和插件。 |

当 Nginx UI 限制插件进程时，会取这些提示值和它自身限制中较小的一个，因此提示值只能降低限制，不能提高限制。请留出余量：超出内存限制的进程会被终止。`recommended_memory_mb` 只是建议，Nginx UI 会在选择插件的地方显示它，并在机器内存不足时发出警告，但绝不会因此拒绝安装插件。参见[生命周期](./lifecycle.md#resource-limits)。

## 能力 {#capabilities}

`capabilities` 列出插件实现的能力。每个名称只能出现一次，且必须是以下之一：

| 名称 | 需要的配置块 | 需要的权限 | 页面 |
| --- | --- | --- | --- |
| `dns01` | 至少包含一个服务商的 `dns01` | | [DNS-01](./capabilities/dns01.md) |
| `http` | 包含 `listen` 的 `http` | | [HTTP 接口](./http.md) |
| `notify` | 至少包含一个渠道的 `notify` | | [通知渠道](./capabilities/notify.md) |
| `probe` | 至少包含一种类型的 `probe` | | [健康检查](./capabilities/probe.md) |
| `mcp` | 至少包含一个工具的 `mcp` | `mcp` | [MCP 工具](./capabilities/mcp.md) |
| `storage` | 至少包含一个后端的 `storage` | | [存储](./capabilities/storage.md) |
| `cert.deploy` | 至少包含一个目标的 `deploy` | `cert.deploy` | [证书部署](./capabilities/cert-deploy.md) |
| `security.blocklist` | 至少包含一个来源的 `blocklist` | `network` | [封禁列表](./capabilities/blocklist.md) |
| `upstream.discovery` | 至少包含一个提供方的 `discovery` | `network` | [上游发现](./capabilities/discovery.md) |
| `log.sink` | `log_sink`，可选 | `log.read` | [访问日志推送](./capabilities/log-sink.md) |

所有能力都由进程提供，因此没有 `server` 的清单不能声明能力、`cron` 或 `events`。

一个插件可以实现多个能力，也可以在之后的版本中追加：列出新的名称，加上对应的配置块并实现它的方法。握手必须报告相同的集合，参见[生命周期](./lifecycle.md#handshake)。新能力需要插件原本没有的权限时，更新会等待批准，参见[权限](./permissions.md#granted-and-requested-permissions)。插件不能自定义能力；超出上表范围的功能请使用 [HTTP](./http.md)、[浏览器包](./webapp.md)和[事件](./host-api.md#events)。

## 依赖与冲突 {#dependencies-and-conflicts}

```json [plugin.json]
"requires": [{ "id": "com.nginxui.dns01", "version": ">=1.2.0" }],
"requires_capabilities": ["storage"],
"conflicts": ["io.github.other.mydns"]
```

- `requires` 列出必须先安装并启用的插件。`id` 是插件 ID，`version` 是可选的语义化版本范围。依赖缺失时 Nginx UI 会拒绝启用插件。
- `requires_capabilities` 列出需要由其他已启用插件提供的能力。没有插件提供时 Nginx UI 会发出警告。
- `conflicts` 列出绝不能与本插件同时运行的插件，例如同一功能的另一种实现。每一项都必须是有效的插件 ID，不能是本插件自己的 ID，不能重复，也不能同时出现在 `requires` 中。冲突关系是双向的：任意一方列出另一方，两者就冲突。冲突的插件已启用时，Nginx UI 会拒绝启用本插件，除非用户选择替换它。

## 设置 {#settings}

`settings_schema` 描述由 Nginx UI 为插件渲染的设置表单。设置值会在握手和每次 `plugin.configure` 调用中传给进程，参见[生命周期](./lifecycle.md#settings)。

```json [plugin.json]
"settings_schema": {
  "header": "Defaults for every certificate.",
  "settings": [
    { "key": "timeout", "type": "number", "display_name": "Timeout", "help_text": "Seconds to wait for the API.", "default": 30 },
    { "key": "region", "type": "select", "display_name": "Region", "options": [
      { "value": "eu", "label": "Europe" },
      { "value": "us", "label": "United States" }
    ] },
    { "key": "api_key", "type": "secret", "display_name": "API key", "required": true }
  ]
}
```

| 字段 | 含义 |
| --- | --- |
| `header`、`footer` | 表单上方和下方的文字。 |
| `settings[].key` | 值的键，在清单内唯一。 |
| `settings[].type` | `text`、`bool`、`number`、`select`、`secret`、`textarea` 或 `list`。 |
| `settings[].display_name` | 字段标签。 |
| `settings[].help_text` | 字段下方的说明。 |
| `settings[].default` | 默认值。`list` 类型为字符串数组。 |
| `settings[].options` | `{ "value", "label" }` 选项，`select` 类型必填。 |
| `settings[].required` | 该字段必须填写。 |

`list` 字段保存字符串数组，显示为可编辑的列表。`secret` 字段的值永远不会发回浏览器：表单显示占位内容，原样保存占位内容会保留已存储的值。

带有浏览器包的插件可以用自己的组件替换生成的表单，参见[浏览器包](./webapp.md#settings-panel)。
