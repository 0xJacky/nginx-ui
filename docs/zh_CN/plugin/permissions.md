---
outline: [2, 3]
---

# 权限与安全

## 插件能做什么 {#what-a-plugin-can-do}

::: danger 警告
插件的服务端进程以与 Nginx UI 相同的操作系统用户运行，没有任何沙箱：它可以读取该用户能读取的文件，也可以像其他程序一样建立网络连接。因此安装插件就意味着信任它的发布者，[签名与信任等级](./signing.md)就是帮助人们做这个判断的。
:::

权限是另一回事。权限决定插件可以使用哪些 **Nginx UI 功能**（它的键值存储、已保存的凭据、通知、Nginx UI 的 REST API），以及 Nginx UI 会把哪些数据交给它（证书、访问日志）。权限也会告诉安装插件的人它将要做什么。

每个插件都有一个只有它自己可以写入的数据目录。Nginx UI 施加的资源限制只约束进程消耗多少资源，不约束它能访问什么。

## 权限 {#permissions}

插件在清单的 `permissions` 数组中列出它需要的权限：

| 权限 | 允许的内容 |
| --- | --- |
| `kv` | 键值存储：`host.kv.get`、`set`、`delete` 和 `list`。 |
| `network` | Nginx UI 不强制执行任何限制。它声明插件自身会发起对外网络连接，每个与服务商通信的插件都应请求它。`security.blocklist` 和 `upstream.discovery` 必须请求它。 |
| `cron` | 在运行时用 `host.cron.register` 和 `unregister` 注册定时调用。 |
| `notify` | 用 `host.notify` 在 Nginx UI 中发出通知。 |
| `metrics.read` | 用 `host.metrics.snapshot` 读取 Nginx UI 的指标。 |
| `core_api` | 在浏览器包中通过 `registry.coreHttp` 调用 Nginx UI 的 REST API。 |
| `mcp` | 把插件的工具发布给 MCP 客户端。`mcp` 能力必须请求它。 |
| `cert.deploy` | 在 `deploy.push` 中接收证书及其私钥。`cert.deploy` 能力必须请求它。 |
| `log.read` | 在 `log.push` 中接收每一行 nginx 访问日志。`log.sink` 能力必须请求它。 |
| `log.files` | 用 `host.logs.list` 列出 nginx 日志文件、接收 `log.paths_changed` 事件，并读取这些文件及其轮转副本。 |
| `nginx.snippet` | 用 `host.nginx.snippet.*` 维护 nginx 配置片段。每次变更时 Nginx UI 都会测试配置并重载 nginx。 |
| `nginx.config.read` | 用 `host.nginx.config.list` 和 `get` 读取 nginx 配置文件。 |
| `sites.read` | 用 `host.sites.list` 列出站点。 |
| `certs.read` | 用 `host.certs.list` 列出证书，不包含私钥。 |
| `credentials.read:<kind>` | 用 `host.credentials.get` 读取某一类已保存的凭据，例如 `credentials.read:dns`。 |

`host.log`、`host.settings.get`、`host.i18n.locale` 和 `host.activity.set` 不需要权限。

只请求插件实际用到的权限。检查工具会对没有能力用到的权限发出警告，也会对访问外部服务却没有 `network` 的能力发出警告。

### 已授予与已请求的权限 {#granted-and-requested-permissions}

清单列出插件请求的权限，安装插件的人批准这个列表，只有已批准的权限才算数：调用需要插件未持有的权限时，会以[权限拒绝错误](./protocol.md#error-codes)失败，并且不执行任何操作。

当更新请求了尚未批准的权限时，Nginx UI 不会静默授予。在有人批准新的权限列表之前，更新后的插件保持停止，它的能力也不可用。只减少权限的更新不需要批准；被去掉的权限如果在之后的版本中再次请求，需要重新批准。因此运行中的插件总是持有清单请求的全部权限。握手会告诉进程它持有哪些权限，参见[生命周期](./lifecycle.md#handshake)。

### 敏感权限 {#sensitive-permissions}

有些权限会把需要格外小心的数据交给插件，Nginx UI 在请求批准时会明确说明：

- **`mcp`** 允许 Nginx UI 授权的每个 MCP 客户端运行插件的工具。请把工具参数视为不可信的输入。
- **`cert.deploy`** 会把用户绑定到插件目标的每个证书的私钥交给插件。请把私钥当作凭据对待，调用结束后不要保留。
- **`log.read`** 和 **`log.files`** 会把 nginx 日志交给插件：客户端地址、每个请求的 URL 及其查询字符串、来源页和用户代理。在许多司法管辖区，这些都属于个人数据。请把每个字段视为不可信的输入，绝不要写入自己的日志。
- **`nginx.snippet`** 让插件在用户引入其片段的位置改变 nginx 提供的服务。Nginx UI 只在整体配置保持有效时才应用片段，但引入片段的人把这部分配置交给了插件。请在要求用户引入片段之前，先说明它的作用。
- **`security.blocklist`** 和 **`upstream.discovery`** 让插件决定 nginx 提供什么服务。Nginx UI 会解析插件返回的每个地址和端口，绝不会把其他文本复制到 nginx 配置中，但依赖封禁列表的人仍然需要信任其插件不会把他们自己拒之门外。

## 声明访问的主机 {#declaring-network-hosts}

`network_hosts` 列出插件打算访问的主机：

```json [plugin.json]
"network_hosts": ["api.mydns.example"]
```

Nginx UI 会把这个列表展示给安装插件的人。它仅供参考，不会阻止访问其他主机。

## 说明权限用途 {#explaining-permissions}

`permission_reasons` 用一句话说明插件为什么需要某项权限，让用户可以对照插件的实际功能来判断：

```json [plugin.json]
"permissions": ["log.files", "network"],
"permission_reasons": {
  "log.files": "To index the access and error logs of your sites for search and the dashboard.",
  "network": "To download the IP location database when you choose to."
},
"i18n": {
  "zh_CN": {
    "permission_reasons": { "network": "在你选择下载时获取 IP 归属地数据库。" }
  }
}
```

凡是请用户授予权限的地方，包括市场、安装对话框和新权限的审批，Nginx UI 都会在权限下方以作者说明的形式显示这段原因。没有写原因或原因为空的权限只显示 Nginx UI 自己的说明。某个语言的翻译为空时，显示英文原因。

每个键都必须是 `permissions` 中的一项，原因最多 300 个字符。键不在其中或原因过长时，[`nginx-ui plugin lint`](./rules.md#manifest-permission-reasons) 会报告，Nginx UI 也会拒绝安装这样的 manifest。请说明用途，而不是解释权限本身：每项权限允许做什么，Nginx UI 已经有说明。

## 处理凭据 {#handling-credentials}

::: danger 警告
凭据指用户以机密形式输入的任何值（DNS 服务商的凭据字段、`secret` 类型的设置、渠道、目标或后端中 `secret` 类型的配置字段）、`host.credentials.get` 返回的任何内容，以及 `deploy.push` 携带的私钥。凭据绝不能出现在：

- 返回给 Nginx UI 的错误消息中。请改用配置无效错误及其 `field` 详情指出字段；
- `host.log` 调用中；
- 插件的标准错误中。
:::

如果有助于调试，可以记录一个值已收到的遮蔽形式，例如它的长度或最后四个字符，而不是什么都不记录。Go SDK 的 `sdk.Redact` 和 Rust SDK 的 `redact` 就是做这个的。

::: warning 注意
请把能力 `config` 中的每个值都当作机密，而不仅仅是标记为 `secret` 的字段：Webhook 地址中常常包含令牌。
:::

通过环境变量把凭据传给库的插件（很多 DNS 库从环境变量读取凭据）必须在调用结束后立即恢复原来的值，使使用不同凭据的两次调用互不可见，并且绝不能把它们写入任何持久化位置。

Nginx UI 也遵守同样的要求：它加密存储机密，绝不把已存储的机密发回浏览器，也绝不在日志中写入机密的值。
