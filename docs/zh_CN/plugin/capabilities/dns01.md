---
outline: [2, 3]
---

# DNS-01

`dns01` 插件为某个 DNS 服务商完成 ACME DNS-01 验证：签发证书时通过服务商的 API 发布 `_acme-challenge` TXT 记录，完成后再删除它。它的服务商会出现在 Nginx UI 的证书表单和 DNS 凭据编辑器中。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `dns01.present` | 是 | 发布验证用的 TXT 记录。 |
| `dns01.cleanup` | 是 | 删除 `present` 发布的内容。 |
| `dns01.validate` | 否 | 在不发布任何内容的情况下检查凭据。 |
| `dns01.options` | 否 | 报告服务商的传播时间。 |
| `dns01.check` | 否 | 报告记录是否已传播。 |

插件没有实现的可选方法回复 `-32002`（不支持），Nginx UI 会改用自己的行为。

## 声明服务商 {#declaring-providers}

```json [plugin.json]
"capabilities": ["dns01"],
"permissions": ["network"],
"dns01": {
  "providers": [
    {
      "name": "MyDNS",
      "code": "mydns",
      "links": { "api": "https://mydns.example/docs/api" },
      "propagation_timeout_seconds": 120,
      "polling_interval_seconds": 2,
      "form": {
        "fields": [
          { "key": "MYDNS_API_TOKEN", "label": "API token", "group": "credential", "secret": true },
          { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
        ]
      }
    }
  ]
}
```

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `name` | 是 | DNS 服务商的名称。 |
| `code` | 是 | 服务商的标识，参见[命名规则](../naming.md#provider-and-kind-codes)。在清单内唯一。 |
| `links.api` | 否 | 服务商 API 文档的链接。 |
| `propagation_timeout_seconds` | 否 | Nginx UI 等待记录传播的时长。 |
| `polling_interval_seconds` | 否 | 检查传播的间隔。 |
| `form` | 是 | 服务商接受的值。参见[凭据表单](#credential-form)。 |

## 发布记录 {#publishing-the-record}

```json
{
  "jsonrpc": "2.0", "id": 10, "method": "dns01.present",
  "params": {
    "provider": "mydns",
    "config": { "MYDNS_API_TOKEN": "tok_live_xxx", "MYDNS_TTL": "120" },
    "options": { "credential_id": "7", "disable_cname": false },
    "domain": "example.com",
    "fqdn": "_acme-challenge.example.com.",
    "effective_fqdn": "_acme-challenge.example.com.",
    "value": "gfj9Xq...Rg85nM",
    "token": "evaGxfADs...62jcerQ",
    "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7",
    "dry_run": false
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `provider` | 服务商的 `code`。 |
| `config` | 凭据的值：表单两组字段合并成的一个映射，以字段 `key` 为键。 |
| `options` | 证书在验证表单中的选项。参见[证书选项](#certificate-options)。 |
| `domain` | 正在验证的域名，不带开头的 `*.`。 |
| `fqdn` | `_acme-challenge` 名称。 |
| `effective_fqdn` | 记录的实际位置：跟随了 CNAME 时为 CNAME 的目标，否则等于 `fqdn`。 |
| `value` | TXT 记录的值。 |
| `token`、`key_auth` | ACME 验证令牌和密钥授权，供需要自行计算 `value` 的插件使用。 |
| `dry_run` | 为 `true` 时只检查输入，不要访问服务商，也不要修改任何记录。 |

记录发布后回复 `{}`。失败时回复错误：

- 原因是用户输入的内容（例如令牌被拒绝）时，回复 `-32003`，并用 `data.field` 指出字段；
- 其他情况（例如服务商故障）回复 `-32000`。

::: warning 注意
绝不要让 `config` 中的值出现在日志或错误消息中。
:::

## 删除记录 {#removing-the-record}

`dns01.cleanup` 的参数与 `dns01.present` 相同，删除为同一 `effective_fqdn` 和 `value` 发布的记录，并回复 `{}`。签发结束时 Nginx UI 总会调用它，所以在没有可删除的内容时（包括 `present` 从未运行或已失败时）它也必须成功。

## 校验凭据 {#validating-a-credential}

```json
{ "jsonrpc": "2.0", "id": 11, "method": "dns01.validate", "params": { "provider": "mydns", "config": { "MYDNS_API_TOKEN": "" } } }
```

在不访问服务商的情况下检查这些值：对第一个缺失或格式错误的字段回复带 `data.field` 的 `-32003`，值看起来可用时回复 `{}`。用户填写凭据表单时 Nginx UI 会调用它。

## 传播时间 {#propagation-timing}

::: code-group

```json [请求]
{ "jsonrpc": "2.0", "id": 12, "method": "dns01.options", "params": { "provider": "mydns", "config": {}, "options": {} } }
```

```json [响应]
{ "jsonrpc": "2.0", "id": 12, "result": { "propagation_timeout_seconds": 120, "polling_interval_seconds": 2, "sequential_interval_seconds": 0 } }
```
:::

`sequential_interval_seconds` 不为 `0` 时，表示服务商无法同时处理同一账户的两个验证，调用之间必须至少间隔这么多秒。没有这个方法时，Nginx UI 使用清单中的时间，再退回到自己的默认值。

## 检查传播 {#checking-propagation}

::: code-group

```json [请求]
{
  "jsonrpc": "2.0", "id": 13, "method": "dns01.check",
  "params": { "provider": "mydns", "config": {}, "options": {}, "domain": "example.com", "fqdn": "_acme-challenge.example.com.", "value": "gfj9Xq...Rg85nM", "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7" }
}
```

```json [响应]
{ "jsonrpc": "2.0", "id": 13, "result": { "ready": false, "effective_fqdn": "_acme-challenge.example.com.", "detail": "NXDOMAIN from ns1.example.com" } }
```
:::

`ready` 表示记录是否在插件检查的所有位置都已可见，`effective_fqdn` 是它查询的名称，`detail` 说明尚未就绪的原因。当插件能比公共 DNS 更快或更可靠地检查时（例如通过服务商的 API），才需要实现这个方法。没有它时，Nginx UI 会自己查询 DNS。

## 证书选项 {#certificate-options}

`options` 携带验证表单与证书一起保存的内容。它对 Nginx UI 是不透明的：插件的[验证表单组件](../slots.md#certificate-challenge-form)写入 `challenge_config` 的内容会原样返回。官方 DNS-01 插件使用：

| 键 | 含义 |
| --- | --- |
| `credential_id` | 所选凭据的 ID。 |
| `disable_cname` | 不跟随验证记录的 CNAME。 |
| `disable_authoritative_ns_propagation` | 跳过对权威域名服务器的检查。 |
| `disable_recursive_ns_propagation` | 跳过对递归域名服务器的检查。 |

`options` 可能为空，例如插件还没有表单时签发的证书。此时请使用合理的默认值，而不是直接失败。

## 凭据表单 {#credential-form}

`form` 描述服务商接受的每个值。Nginx UI 只根据它构建凭据表单，绝不会保存或发送表单没有列出的键。没有任何值的服务商声明 `"fields": []`。

```json [plugin.json]
"form": {
  "fields": [
    { "key": "MYDNS_API_TOKEN", "label": "API token", "help": "Needs the DNS edit permission.", "group": "credential", "secret": true },
    { "key": "MYDNS_API_EMAIL", "label": "Account email", "group": "credential" },
    { "key": "MYDNS_API_KEY", "label": "API key", "group": "credential", "secret": true },
    { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
  ],
  "methods": [
    { "name": "API token", "recommended": true, "fields": ["MYDNS_API_TOKEN"] },
    { "name": "Global API key", "fields": ["MYDNS_API_EMAIL", "MYDNS_API_KEY"] },
    { "name": "Instance role", "fields": [], "values": { "MYDNS_AUTH_MODE": "instance" } }
  ]
}
```

### 字段 {#fields}

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `key` | 是 | 值在 `config` 中的键，在表单内唯一。 |
| `label` | 是 | 简短的标签。 |
| `help` | 否 | 显示在输入框下方的一句说明。 |
| `group` | 是 | `credential` 表示登录用的值，`setting` 表示调优项，例如 TTL、基础地址或超时。 |
| `optional` | 否 | 没有这个值服务商也能工作。 |
| `secret` | 否 | 密码、令牌或密钥，显示为密码输入框。 |
| `default` | 否 | 字段为空时使用的值。显示为占位内容，不会替用户保存。 |
| `unit` | 否 | 秒数时为 `seconds`。 |
| `link` | 否 | 字段的文档链接。 |

Nginx UI 会把 `setting` 字段与凭据分开显示，并分开保存两组值，其中凭据按机密处理。插件收到的 `config` 中两组值是合并在一起的。为空或为 `false` 的属性请省略，以保持清单简洁。

### 登录方式 {#sign-in-methods}

服务商提供多种登录方式时，`methods` 列出它们，至少两项：

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `name` | 是 | 方式的名称，唯一。 |
| `recommended` | 否 | 默认选中这种方式，最多一项。 |
| `fields` | 是 | 这种方式使用的 `credential` 字段的键。不需要输入的方式为空。 |
| `values` | 否 | 在插件中选定这种方式的固定值，例如认证模式。 |

- 没有被任何方式列出的凭据字段是共享的，每种方式下都会显示。Nginx UI 只显示所选方式的字段和共享字段，保存时清除所选方式不使用的凭据值。
- `values` 会与凭据一起保存，并在 `config` 中发送。`values` 的键通常不是字段；它也可以是某些方式列出、另一些方式固定取值的凭据字段：列出它的方式显示该字段，设置它的方式固定它的值。不能有一种方式既列出又设置同一个键。
- 任意两种方式的字段和值不能完全相同。

编辑已保存的凭据时，Nginx UI 预先选中 `values` 与保存值匹配、且字段都有值的方式；否则选中推荐的方式；再否则选中第一种。

### 翻译 {#translations}

服务商 `name`、每个 `label` 和 `help` 以及每种方式的 `name` 都是英文原文。Nginx UI 会用插件浏览器包通过 `registerTranslations` 注册的翻译来翻译它们，没有翻译时显示英文。

违反本节任何规则的表单无法通过 [`dns01-form`](../rules.md#dns01-form) 检查，Nginx UI 可能会拒绝安装该插件。
