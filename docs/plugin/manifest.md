---
outline: [2, 3]
---

# Manifest

Every plugin has a `plugin.json` at the root of its package. It is a single
UTF-8 JSON object that says what the plugin is, what it contains and what it
asks for. Nginx UI validates it before it installs a plugin and refuses a
package whose manifest breaks any rule on this page.

The [JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/plugin.schema.json)
of the manifest gives editors completion and validation:

```json
{
  "$schema": "https://raw.githubusercontent.com/nginxui/plugin-spec/main/schema/plugin.schema.json",
  "id": "io.github.example.mydns"
}
```

## Fields

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `id` | string | yes | Unique plugin id. See [Identity](#identity). |
| `name` | string | yes | Display name. |
| `version` | string | yes | The plugin's own version, [semantic versioning](https://semver.org/). |
| `description` | string | no | One line summary shown in the plugin list. |
| `i18n` | object | no | Translations of `name`, `description`, `permission_reasons` and the screenshot captions. See [Translations](#translations). |
| `homepage_url` | string | no | Documentation or repository link. |
| `icon_path` | string | no | Path of an icon file inside the package. |
| `api_version` | integer | yes | Protocol generation the plugin speaks. Always `1` today. |
| `min_nginx_ui_version` | string | no | Oldest Nginx UI version the plugin supports. See [Versions and Compatibility](./versioning.md). |
| `server` | object | no\* | The server process. See [Server Process](#server-process). |
| `webapp` | object | no\* | The browser bundle or static pages. See [Browser Bundle](./webapp.md). |
| `content` | object | no\* | Templates and translation files. See [Templates and Translations](./capabilities/content.md). |
| `capabilities` | string[] | no | Capabilities the plugin implements. See [Capabilities](#capabilities). |
| `permissions` | string[] | no | Nginx UI features the plugin needs. See [Permissions and Security](./permissions.md). |
| `requires` | object[] | no | Plugins this plugin depends on. See [Dependencies and Conflicts](#dependencies-and-conflicts). |
| `requires_capabilities` | string[] | no | Capabilities some other enabled plugin must provide. |
| `conflicts` | string[] | no | Plugins that must never run together with this one. |
| `events` | string[] | no | Events the plugin subscribes to. See [Host API](./host-api.md#events). |
| `cron` | object[] | no | Scheduled calls. See [Host API](./host-api.md#scheduled-tasks). |
| `network_hosts` | string[] | no | Hosts the plugin intends to contact, shown to the person installing it. |
| `permission_reasons` | object | no | Why the plugin asks for each permission. See [Explaining Permissions](./permissions.md#explaining-permissions). |
| `screenshots` | object[] | no | Images of the plugin in use for its catalog listing. See [Screenshots](#screenshots). |
| `settings_schema` | object | no | The settings form. See [Settings](#settings). |
| `dns01`, `http`, `notify`, `probe`, `mcp`, `storage`, `deploy`, `blocklist`, `discovery`, `log_sink` | object | with the capability | Configuration of each capability. |

\* A manifest declares at least one of `server`, `webapp` and `content`, since
a plugin with none of them would install nothing.

::: info
Every path in a manifest (`icon_path`, `server.executables`, `webapp` paths,
`content` paths) is relative to the package root and must be a
[safe relative path](./packaging.md#safe-paths). Screenshot paths are the one
exception: they are relative to the repository, see [Screenshots](#screenshots).
:::

## Identity

- `id` is lowercase letters, digits and hyphens in two or more dot separated
  segments, at most 64 characters: `^[a-z0-9]+(\.[a-z0-9-]+)+$`, for example
  `io.github.example.mydns`. The id never changes over the life of a plugin.
  [Naming](./naming.md#plugin-ids) explains how to choose one.
- `name` is required and not empty.
- `version` follows semantic versioning 2.0.0, such as `1.4.0` or
  `2.0.0-beta.1`. Prerelease versions put a release on a less stable
  [channel](./catalog.md#release-channels).
- `api_version` is `1`. Nginx UI refuses a plugin with any other value.

## Translations

`i18n` translates the name and description into the languages of the Nginx UI
interface:

```json [plugin.json]
"i18n": {
  "zh_CN": { "name": "DNS-01 验证", "description": "使用任意 DNS 服务商完成 ACME DNS-01 验证。" },
  "ja_JP": { "name": "DNS-01 チャレンジ" }
}
```

Translate only the languages you want to. The top level `name` and
`description` are the English text, and a language without a translation
shows them instead. A language can translate
[`permission_reasons`](./permissions.md#explaining-permissions) the same way.

Keys are the language codes of the Nginx UI interface, such as `zh_CN`,
`zh_TW` or `ja_JP`. Nginx UI shows the languages its interface has and the
English text for the others, so a plugin can carry a language before every
Nginx UI has it. [`nginx-ui plugin lint`](./rules.md#manifest-i18n) reports a
key that is not a language code, and warns about a language the Nginx UI it
runs with does not have.

## Screenshots

`screenshots` lists up to eight images of the plugin in use, shown on its
catalog listing in this order:

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

Unlike the other paths of a manifest, `path` and `dark_path` are relative to
the root of the plugin repository, not to the package, so the images stay out
of the package. The catalog reads them at the tag of each release, so a new
release brings new screenshots. Use PNG, JPEG or WebP images, about 16:10 at
1280 to 1920 pixels wide. `dark_path` is the same view in the dark theme, shown
while the Nginx UI interface is dark. `id` names the
screenshot, lowercase letters, digits and hyphens. `caption` is English, and
`screenshot_captions` in `i18n` translates it, keyed by `id`.
[`nginx-ui plugin lint`](./rules.md#manifest-screenshots) checks the paths and
captions.

## Server Process

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

| Field | Meaning |
| --- | --- |
| `executables` | Platform key (`<goos>-<goarch>`, see [Naming](./naming.md#platform-keys)) to the executable for that platform. |
| `command` | Command line for an interpreted plugin, such as `["python3", "server/main.py"]`. Used when `executables` has no entry for the platform. |
| `lifecycle` | `resident` (default) keeps the process running while the plugin is enabled. `on_demand` starts it when it is needed. |
| `idle_timeout_seconds` | For `on_demand`: how long the process may stay idle before Nginx UI stops it. Not negative. |
| `resources` | What the process needs. See [Resources](#resources). |

`server` needs `executables` or `command`. The first element of `command` is
either a program looked up on `PATH`, such as `python3`, or a path inside the
package. Nginx UI marks every declared executable as executable when it
installs the package, and fails to start the plugin when the platform it runs
on has neither an executable nor a command.

[Lifecycle](./lifecycle.md) describes how the process is started, checked and
stopped.

### Resources

`server.resources` tells Nginx UI and the person installing the plugin what
the process needs. All three values are optional and never negative.

| Field | Meaning |
| --- | --- |
| `memory_mb` | Most memory the process uses, in MiB. |
| `cpu_percent` | Most CPU time the process uses, in percent of one core: `100` is one core, `250` two and a half. |
| `recommended_memory_mb` | Memory the machine (or the container Nginx UI runs in) should have, counting Nginx UI and the plugin together. |

When Nginx UI limits plugin processes, it applies the smaller of these hints
and its own limit, so a hint can lower a limit but never raise it. Leave
headroom: a process that exceeds its memory limit is killed.
`recommended_memory_mb` is advice only. Nginx UI shows it where a plugin is
chosen and warns when the machine has less, but never refuses to install the
plugin because of it. See [Lifecycle](./lifecycle.md#resource-limits).

## Capabilities

`capabilities` lists what the plugin implements. Each name appears once and
must be one of:

| Name | Required block | Required permissions | Page |
| --- | --- | --- | --- |
| `dns01` | `dns01` with at least one provider | | [DNS-01](./capabilities/dns01.md) |
| `http` | `http` with `listen` | | [HTTP Endpoints](./http.md) |
| `notify` | `notify` with at least one channel | | [Notification Channels](./capabilities/notify.md) |
| `probe` | `probe` with at least one kind | | [Health Checks](./capabilities/probe.md) |
| `mcp` | `mcp` with at least one tool | `mcp` | [MCP Tools](./capabilities/mcp.md) |
| `storage` | `storage` with at least one backend | | [Storage](./capabilities/storage.md) |
| `cert.deploy` | `deploy` with at least one target | `cert.deploy` | [Certificate Deployment](./capabilities/cert-deploy.md) |
| `security.blocklist` | `blocklist` with at least one source | `network` | [Blocklists](./capabilities/blocklist.md) |
| `upstream.discovery` | `discovery` with at least one provider | `network` | [Upstream Discovery](./capabilities/discovery.md) |
| `log.sink` | `log_sink`, optional | `log.read` | [Access Log Streaming](./capabilities/log-sink.md) |

Every capability is served by the process, so a manifest without `server`
declares no capabilities, no `cron` and no `events`.

A plugin may implement several capabilities and add more in a later version:
list the new name, add its block and implement its methods. The handshake
must report the same set, see [Lifecycle](./lifecycle.md#handshake). When the
new capability needs a permission the plugin did not have, the update waits
for approval, see [Permissions](./permissions.md#granted-and-requested-permissions).
A plugin cannot define capabilities of its own; for features outside this
list, use [HTTP](./http.md), a [browser bundle](./webapp.md) and
[events](./host-api.md#events).

## Dependencies and Conflicts

```json [plugin.json]
"requires": [{ "id": "com.nginxui.dns01", "version": ">=1.2.0" }],
"requires_capabilities": ["storage"],
"conflicts": ["io.github.other.mydns"]
```

- `requires` lists plugins that must be installed and enabled first. `id` is
  a plugin id and `version` an optional semantic version range. Nginx UI
  refuses to enable a plugin whose dependencies are missing.
- `requires_capabilities` lists capabilities some other enabled plugin must
  provide. Nginx UI warns when none does.
- `conflicts` lists plugins that must never run at the same time as this one,
  for example an alternative implementation of the same feature. Each entry is
  a valid plugin id, not the plugin's own id, not listed twice and not also in
  `requires`. The relation is mutual: two plugins conflict when either lists
  the other. Nginx UI refuses to enable a plugin while a conflicting one is
  enabled, unless the person chooses to replace it.

## Settings

`settings_schema` describes a settings form that Nginx UI renders for the
plugin. The values reach the process in the handshake and in every
`plugin.configure` call, see [Lifecycle](./lifecycle.md#settings).

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

| Field | Meaning |
| --- | --- |
| `header`, `footer` | Text above and below the form. |
| `settings[].key` | Key of the value. Unique within the manifest. |
| `settings[].type` | `text`, `bool`, `number`, `select`, `secret`, `textarea` or `list`. |
| `settings[].display_name` | Field label. |
| `settings[].help_text` | Description under the field. |
| `settings[].default` | Default value. For `list`, an array of strings. |
| `settings[].options` | `{ "value", "label" }` choices, required for `select`. |
| `settings[].required` | The field must be filled in. |

A `list` field holds an array of strings and is shown as an editable list. A
`secret` field is never sent back to the browser: the form shows a
placeholder, and saving the placeholder unchanged keeps the stored value.

A plugin with a browser bundle can replace the generated form with its own
component, see [Browser Bundle](./webapp.md#settings-panel).
