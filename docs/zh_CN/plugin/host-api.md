---
outline: [2, 3]
---

# 宿主 API

握手完成后，插件可以通过 `host.*` 方法调用 Nginx UI。它们是插件发给 Nginx UI 的普通 JSON-RPC 请求，通过标准输入输出传输。Nginx UI 也会主动调用插件，用于已订阅的事件和定时任务。

| 方法 | 权限 | 用途 |
| --- | --- | --- |
| `host.log` | 无 | 写入一条结构化日志。 |
| `host.kv.get`、`set`、`delete`、`list` | `kv` | 使用插件的键值存储。 |
| `host.settings.get` | 无 | 读取插件当前的设置。 |
| `host.i18n.locale` | 无 | 读取界面语言。 |
| `host.credentials.get` | `credentials.read:<kind>` | 读取已保存的凭据。 |
| `host.cron.register`、`unregister` | `cron` | 在运行时安排定时调用。 |
| `host.notify` | `notify` | 在 Nginx UI 中发出通知。 |
| `host.metrics.snapshot` | `metrics.read` | 读取 Nginx UI 的指标。 |
| `host.logs.list` | `log.files` | 列出插件可以读取的 nginx 日志文件。 |
| `host.activity.set` | 无 | 在处理指示器中显示后台工作。 |
| `host.nginx.snippet.put`、`delete`、`list` | `nginx.snippet` | 维护 nginx 配置片段。 |
| `host.nginx.config.list`、`get` | `nginx.config.read` | 读取 nginx 配置文件。 |
| `host.sites.list` | `sites.read` | 列出站点。 |
| `host.certs.list` | `certs.read` | 列出证书。 |

::: info 说明
只有在 `plugin.initialized` 之后，并且只有插件实际持有相应权限时，才能调用这些方法（参见[权限与安全](./permissions.md#granted-and-requested-permissions)）。没有权限的调用会以 `-32001` 失败，并且不执行任何操作。
:::

## 日志 {#logging}

```json
{ "jsonrpc": "2.0", "id": 20, "method": "host.log", "params": { "level": "info", "message": "reconfigured", "fields": { "settings_count": 2 } } }
```

`level` 为 `debug`、`info`、`warn` 或 `error`，`fields` 是可选的结构化上下文。Nginx UI 回复 `{}`，并把这一行与插件的其他日志输出一起显示。写到标准错误的内容也会出现在那里。绝不要记录凭据。

## 键值存储 {#key-value-store}

每个插件都有自己的键值存储。键是非空字符串，值可以是任意 JSON 值，编码后最大 **64 KiB**。

::: code-group

```json [请求]
{ "jsonrpc": "2.0", "id": 21, "method": "host.kv.set", "params": { "key": "last_sync", "value": { "at": 1732000000 } } }
{ "jsonrpc": "2.0", "id": 22, "method": "host.kv.get", "params": { "key": "last_sync" } }
```

```json [响应]
{ "jsonrpc": "2.0", "id": 22, "result": { "value": { "at": 1732000000 }, "found": true } }
```
:::

| 方法 | 参数 | 结果 |
| --- | --- | --- |
| `host.kv.get` | `key` | `value` 和 `found`。键不存在时 `found: false`，不算错误。 |
| `host.kv.set` | `key`、`value` | `{}`。值过大时以 `-32602` 失败。 |
| `host.kv.delete` | `key` | `{}`，无论键是否存在。 |
| `host.kv.list` | 可选的 `prefix` | `keys` 数组，没有匹配时为空数组。 |

两个插件永远看不到彼此的键。较大或结构化的数据请改用插件数据目录中的文件。

## 设置与语言 {#settings-and-language}

`host.settings.get` 返回最近一次握手或 `plugin.configure` 中的设置，插件无需自己保存一份：

```json
{ "jsonrpc": "2.0", "id": 23, "result": { "settings": { "timeout": 120 } } }
```

`host.i18n.locale` 返回界面语言，例如 `{ "locale": "zh_CN" }`，可用于本地化通知或日志消息。

## 凭据 {#credentials}

`host.credentials.get` 按类型和 ID 读取 Nginx UI 中保存的凭据：

::: code-group

```json [请求]
{ "jsonrpc": "2.0", "id": 25, "method": "host.credentials.get", "params": { "kind": "dns", "id": "7" } }
```

```json [响应]
{ "jsonrpc": "2.0", "id": 25, "result": { "id": "7", "name": "Cloudflare - example.com", "provider_code": "cloudflare", "config": { "CF_DNS_API_TOKEN": "..." } } }
```
:::

权限中写明了类型，例如 `credentials.read:dns`，并且只覆盖该类型。DNS-01 插件很少需要这个方法：证书使用的凭据已经在每次调用中传入。

## 定时任务 {#scheduled-tasks}

插件可以让 Nginx UI 按计划调用它的某个方法。固定的计划在清单中声明，不需要权限：

```json [plugin.json]
"cron": [
  { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" }
]
```

也可以在持有 `cron` 权限时于运行时注册：

```json
{ "jsonrpc": "2.0", "id": 26, "method": "host.cron.register", "params": { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" } }
```

- `schedule` 是五段式 cron 表达式或 `@every <时长>`。
- `method` 是插件提供的方法，最好使用插件自己的名称，例如 `catalog.refresh`。
- 再次注册同一个 `id` 会替换之前的条目。`host.cron.unregister` 使用 `{ "id": ... }` 删除条目，即使条目不存在也回复 `{}`。

计划触发时，Nginx UI 向该方法发送一个普通请求：

```json
{ "jsonrpc": "2.0", "id": 31, "method": "catalog.refresh", "params": { "type": "refresh-catalog", "ts": 1732000000 } }
```

`type` 是条目 ID，`ts` 是触发时的 Unix 时间。请回复这个请求；结果会被忽略，错误会被记录但不会重试。每次调用 Nginx UI 最多等待 **10 分钟**，会为此启动 `on_demand` 插件，并且在上一次调用仍在运行时不会再次触发同一条目。

## 通知 {#notifications}

```json
{ "jsonrpc": "2.0", "id": 27, "method": "host.notify", "params": { "level": "warning", "title": "Propagation slow", "content": "example.com is taking longer than usual to propagate.", "details": { "domain": "example.com" } } }
```

`level` 为 `info`、`success`、`warning` 或 `error`。通知会出现在 Nginx UI 的通知中，`details` 是按需显示的可选上下文。Nginx UI 把通知加入队列后回复 `{}`。

`notify` 权限与 `notify` 能力无关，后者让插件负责发送通知，参见[通知渠道](./capabilities/notify.md)。

## 指标 {#metrics}

`host.metrics.snapshot` 返回 Nginx UI 当前的指标，例如 `{ "snapshot": { "cpu_percent": 3.2, "memory_bytes": 104857600 } }`。其结构不保证在各版本之间保持不变，请把它当作尽力而为的遥测数据。

## 日志文件 {#log-files}

持有 `log.files` 权限的插件可以自己读取 nginx 日志文件。`host.logs.list` 返回 Nginx UI 允许读取的文件：

```json
{ "jsonrpc": "2.0", "id": 29, "result": { "logs": [
  { "path": "/var/log/nginx/access.log", "type": "access", "source": "default" },
  { "path": "/var/log/nginx/example.com.error.log", "type": "error", "source": "config", "config_file": "/etc/nginx/sites-enabled/example.com.conf" }
] } }
```

| 字段 | 含义 |
| --- | --- |
| `path` | 当前日志文件的绝对路径。 |
| `type` | `access` 或 `error`。 |
| `source` | nginx 配置中指定了该路径时为 `config`，nginx 内置默认日志为 `default`。 |
| `config_file` | 指定该路径的配置文件。 |

轮转后的文件（`access.log.1`、`access.log.2.gz`、`access.log-20260101`）不会列出，请在已列出路径的旁边查找它们。只读取已列出的文件及其轮转副本，把内容视为个人数据，绝不写入自己的日志。nginx 配置变化时列表也会变化：请订阅 `log.paths_changed`，收到后再次调用 `host.logs.list`。

如果想在 nginx 写入时接收新日志行而不是读取文件，参见[访问日志推送](./capabilities/log-sink.md)。

## nginx 配置 {#nginx-configuration}

### 配置片段 {#snippets}

持有 `nginx.snippet` 权限的插件可以维护属于自己的 nginx 配置，例如缓存规则或限流。`host.nginx.snippet.put` 写入一个片段：

::: code-group

```json [请求]
{ "jsonrpc": "2.0", "id": 31, "method": "host.nginx.snippet.put", "params": { "name": "static-cache", "content": "location ~* \\.(css|js)$ {\n  expires 7d;\n}\n" } }
```

```json [响应]
{ "jsonrpc": "2.0", "id": 31, "result": { "changed": true, "include": "include snippets/plugins/io.github.example.cache/static-cache.conf;" } }
```

:::

- `name` 为 1 到 64 个 `[a-z0-9_-]` 字符，以字母或数字开头；`content` 最多 256 KiB 的 UTF-8 文本。每个插件最多保留 32 个片段。
- Nginx UI 写入片段，测试整体配置并重载 nginx。nginx 拒绝配置时，之前的片段保持不变，不会重载，调用以 `-32602` 失败并附上 nginx 的输出。
- 片段内容没有变化时 `changed` 为 `false`，此时也不会重载。

片段只在被引入的位置生效。请把 `include` 展示给用户（例如在你的设置面板中），由用户把它加到合适的 `server` 或 `location` 块中。Nginx UI 永远不会自行添加它。

`host.nginx.snippet.list` 返回 `{ "snippets": [{ "name": "...", "include": "..." }] }`；`host.nginx.snippet.delete` 传入 `{ "name": "..." }` 删除一个片段，并回复 `{ "removed": true }`。仍被引入的片段无法删除：否则 nginx 会拒绝配置，因此片段保留，调用以 `-32602` 失败。

::: info 说明
停用插件时片段会保留，nginx 继续提供原有服务。卸载插件时，Nginx UI 会删除它的片段；仍被引入的片段会被清空，以保持配置有效。
:::

### 读取配置 {#reading-the-configuration}

持有 `nginx.config.read` 时，`host.nginx.config.list` 返回相对于 nginx 配置目录的配置文件，例如 `["conf.d/gzip.conf", "nginx.conf", "sites-available/example.com"]`；`host.nginx.config.get` 传入 `{ "path": "nginx.conf" }` 返回 `{ "content": "..." }`。列表包含 `nginx.conf`、所有 `.conf` 文件以及 `sites-available` 和 `streams-available` 中的文件。密钥、密码文件和符号链接不在其中，单个文件最大 1 MiB。

## 站点与证书 {#sites-and-certificates}

`host.sites.list`（`sites.read`）返回站点：

```json
{ "sites": [ { "name": "example.com", "status": "enabled", "urls": ["https://example.com"], "config_file": "sites-available/example.com" } ] }
```

`status` 为 `enabled`、`disabled` 或 `maintenance`，`config_file` 可以用 `host.nginx.config.get` 读取。

`host.certs.list`（`certs.read`）返回证书，从不包含私钥：

```json
{ "certs": [ { "id": "3", "name": "example.com", "domains": ["example.com", "www.example.com"], "auto_renew": true, "challenge_method": "dns01", "key_type": "P256", "not_before": "2026-09-01T00:00:00Z", "not_after": "2026-11-30T00:00:00Z", "issuer": "Let's Encrypt" } ] }
```

证书文件无法读取时，日期和签发者为空。订阅 `site.*` 和 `cert.*` [事件](#events)，以便知道何时重新获取列表。

## 处理指示器 {#processing-indicator}

`host.activity.set` 在 Nginx UI 的处理指示器中显示插件的后台工作，与 Nginx UI 自己的任务并列：

```json
{ "jsonrpc": "2.0", "id": 30, "method": "host.activity.set", "params": { "key": "indexing", "label": "Indexing access logs", "active": true } }
```

- `key` 在插件内标识条目：1 到 64 个 `[a-z0-9._-]` 字符。
- `label` 是 1 到 128 个字符的英文文本。Nginx UI 会用插件浏览器包注册的翻译来翻译它，键就是这段英文。
- `active: true` 显示或更新条目，`active: false` 删除条目。

Nginx UI 为每个插件至少允许 8 个条目，并在插件停止、崩溃或被禁用时删除它的全部条目。

## 事件 {#events}

插件在清单的 `events` 数组中订阅 Nginx UI 的事件：

```json [plugin.json]
"events": ["cert.renewed", "nginx.reload_failed"]
```

Nginx UI 以 `events.on` 通知发送每个事件，通知永远不需要回复：

```json
{ "jsonrpc": "2.0", "method": "events.on", "params": { "type": "cert.renewed", "data": { "domain": "example.com" }, "ts": 1732000000 } }
```

| 事件 | 触发时机 |
| --- | --- |
| `cert.issued` | 证书签发完成。 |
| `cert.renewed` | 证书已续期。 |
| `cert.expiring` | 证书即将过期。 |
| `site.saved` | 网站配置已保存。 |
| `site.enabled` | 网站已启用。 |
| `site.disabled` | 网站已禁用。 |
| `nginx.reloaded` | nginx 已重载。 |
| `nginx.reload_failed` | nginx 重载失败。 |
| `node.status_changed` | 集群节点的状态发生变化。 |
| `node.joined` | 节点加入集群。 |
| `backup.completed` | 备份完成。 |
| `auth.login_failed` | 登录失败。 |
| `plugin.changed` | 插件被安装、更新、启用或禁用。 |
| `log.paths_changed` | `host.logs.list` 返回的文件发生变化。只发送给持有 `log.files` 的插件。 |

只有已订阅的事件才会发送。请忽略不认识的事件类型：将来可能会新增事件。
