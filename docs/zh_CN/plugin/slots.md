---
outline: [2, 3]
---

# 插槽

插槽是 Nginx UI 界面的扩展点。浏览器包用 `registry.registerSlot` 把组件挂载到插槽中：

```ts
registry.registerSlot('certificate.issue.footer', IssueHint, {
  order: 10,
  when: ctx => ctx.options.challenge_method === 'dns01',
})
```

| 选项 | 含义 |
| --- | --- |
| `order` | 多个插件使用同一插槽时，值越小越先渲染。 |
| `when(ctx)` | 返回 `false` 时，在该上下文中不渲染。 |
| `label` | 需要显示文字的插槽使用的标签，是一段英文文本，会用插件的翻译进行翻译。 |
| `sortValue`、`filters` | 日志列表列的排序和筛选。 |

## 属性 {#props}

挂载的组件会收到两份插槽上下文：展开为单独的属性，以及一个 `context` 属性。以下两种写法都可以：

```ts
defineProps<{ options: CertificateOptions }>()
defineProps<{ context: { options: CertificateOptions } }>()
```

每个挂载的组件都有独立的错误边界。组件抛出异常时只显示一条内联错误，不会破坏页面或同一插槽中的其他组件。

## 可用的插槽 {#available-slots}

| 插槽 | 上下文 | 位置 |
| --- | --- | --- |
| `certificate.challenge.form:{method}` | `{ options }` | 某种 ACME 验证方式（例如 `dns01`）的设置。 |
| `certificate.issue.footer` | `{ options }` | 证书签发表单底部。 |
| `dns.credential.form:{provider_code}` | `{ credential, provider }` | 某个服务商的 DNS 凭据表单中的额外字段。 |
| `dns.credential.hint:{provider_code}` | `{ credential, provider }` | 服务商凭据字段上方的内容。 |
| `plugin.settings:{plugin_id}` | `{ settings }` | 另一个插件的设置中。 |
| `sidebar.footer` | `{}` | 侧边栏底部。 |
| `nginx_log.view:{key}` | `{ path, type }` | 某个日志文件的额外视图。 |
| `nginx_log.list.toolbar` | `{ type }` | 日志列表上方的操作区。 |
| `nginx_log.list.column:{key}` | `{ row }` | 日志列表的额外列。 |
| `nginx_log.list.row.actions` | `{ row }` | 日志列表中某一行的操作。 |
| `site.log.actions` | 参见[网站日志操作](#site-log-actions) | 某个网站的日志操作。 |

对 Nginx UI 不认识的插槽名称的注册会被忽略，将来的版本可能会新增插槽。插件为自己定义的插槽以插件 ID 为前缀，参见[命名规则](./naming.md#settings-keys-and-slot-names)。

### 证书验证表单 {#certificate-challenge-form}

`certificate.challenge.form:{method}` 收到 `options`，即正在签发的证书的响应式选项。组件读写 `options.challenge_config`，这个对象属于插件：Nginx UI 会把它和证书一起保存，并在每次 DNS-01 调用中原样作为 `options` 交给插件。参见 [DNS-01](./capabilities/dns01.md#certificate-options)。

## 日志页面插槽 {#log-page-slots}

### 视图 {#views}

每个 `nginx_log.view:{key}` 注册都会在日志页面的切换器中、内置的原始视图旁边添加一种模式，标签为 `opts.label`（没有时使用键名）。页面把所选模式保存在 `view` 查询参数中，因此 `?view=<key>` 可以直接链接到它。只有所选模式的组件会被挂载，并收到文件的 `path` 和 `type`（`access` 或 `error`）。`opts.when(ctx)` 决定是否为某个文件提供该模式。

### 列 {#columns}

每个 `nginx_log.list.column:{key}` 注册都会在内置列之后添加一列，按 `opts.order` 排序，标题为 `opts.label`。组件渲染它收到的那一行的单元格。

`opts.when(ctx)` 对每个列表只调用一次，参数为 `{ type }` 而不是某一行，它决定这一列是否存在。只适用于访问日志的列只需注册一次，并在错误日志列表中隐藏：

```ts
registry.registerSlot('nginx_log.list.column:index_status', StatusCell, {
  label: 'Index Status',
  when: ctx => ctx.type === 'access',
  sortValue: row => statusRank(row.path),
  filters: [{ label: 'Indexed', value: 'indexed', match: row => isIndexed(row.path) }],
})
```

Nginx UI 一次加载整个列表，并在浏览器中对插件列进行排序和筛选：

- 提供 `sortValue(row)` 时，表头按返回值排序：数字按大小，字符串按区域顺序，`null` 和 `undefined` 排在最后。
- 提供 `filters` 时，表头为每一项提供一个选项，标签为 `label`。同一列中选中的选项按“或”组合，不同列之间按“与”组合。`value` 标识选项。

每一行至少包含 `path`、`type`、`name` 和 `config_file`。请忽略不认识的字段。

### 工具栏与行操作 {#toolbar-and-row-actions}

`nginx_log.list.toolbar` 和 `nginx_log.list.row.actions` 按 `opts.order` 渲染每个注册。`opts.when` 收到与组件相同的上下文。

### 网站日志操作 {#site-log-actions}

`site.log.actions` 出现在网站编辑器和网站列表中，收到某个网站的日志文件：

| 属性 | 含义 |
| --- | --- |
| `siteName` | 网站。 |
| `accessLogPath`、`errorLogPath` | 网站自己的 `access_log` 或 `error_log` 路径；没有时为网站回退使用的 nginx 默认日志；都没有时为空字符串。 |
| `accessLogInherited`、`errorLogInherited` | 路径是 nginx 默认日志而不是网站自己的日志时为 `true`。 |

默认日志中可能包含其他网站的流量。针对单个网站流量的操作，在路径是继承来的时候应隐藏自己，或者明确说明这一点。
