---
outline: [2, 3]
---

# 通知渠道

`notify` 插件通过自己的服务发送 Nginx UI 的通知：聊天服务、推送网关、告警系统。它的渠道会出现在内置通知渠道旁边。用户只需配置一次渠道，之后每当有通知路由到该渠道，Nginx UI 就会调用插件。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `notify.send` | 是 | 发送一条通知。 |
| `notify.validate` | 否 | 在不发送任何内容的情况下检查渠道配置。 |

`notify` 能力不是 `notify` 权限：后者允许插件用 `host.notify` 在 Nginx UI 中发出通知。`notify` 插件接收 `notify.send` 不需要任何权限，但由于它要与外部服务通信，应请求 `network`。

## 声明渠道 {#declaring-channels}

```json [plugin.json]
"capabilities": ["notify"],
"permissions": ["network"],
"notify": {
  "channels": [
    {
      "code": "mychat",
      "name": "MyChat",
      "configuration": {
        "fields": [
          { "key": "webhook_url", "display_name": "Webhook URL", "required": true },
          { "key": "token", "display_name": "Token", "help_text": "Bot token of the workspace", "required": true, "secret": true },
          { "key": "mention_all", "type": "bool", "display_name": "Mention everyone on errors" }
        ]
      }
    }
  ]
}
```

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `code` | 是 | 渠道的标识，参见[命名规则](../naming.md#provider-and-kind-codes)。在清单内唯一。 |
| `name` | 是 | 渠道名称。 |
| `configuration.fields` | 否 | 渠道表单的字段。 |

## 配置表单 {#configuration-form}

渠道、健康检查类型、存储后端、部署目标、封禁列表来源和发现提供方都用同样的 `configuration.fields` 描述表单：

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `key` | 是 | 值在 `config` 中的键。不能为空，在表单内唯一。 |
| `type` | 否 | `text`（默认）、`textarea`、`number` 或 `bool`。 |
| `display_name` | 是 | 字段标签。 |
| `help_text` | 否 | 字段下方的说明。 |
| `required` | 否 | 该字段为空时 Nginx UI 不会保存配置。 |
| `secret` | 否 | 凭据：遮蔽显示，从不记录。 |

每个值都以字符串的形式出现在 `config` 中：数字为十进制文本，例如 `"30"`，`bool` 为 `"true"` 或 `"false"`。空字段可能不存在。Nginx UI 会加密保存配置。

::: warning 注意
请把 `config` 中的**每个**值都当作机密，而不仅仅是 `secret` 字段：Webhook 地址中常常包含令牌。
:::

## 发送 {#sending}

```json
{
  "jsonrpc": "2.0", "id": 30, "method": "notify.send",
  "params": {
    "channel": "mychat",
    "config": { "webhook_url": "https://chat.example/hooks/xxx", "token": "tok_live_xxx" },
    "title": "Certificate Expiring Soon",
    "content": "Certificate example.com expires in 7 days.",
    "severity": "warning"
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `channel` | 渠道的 `code`。 |
| `config` | 渠道表单的值。 |
| `title`、`content` | 通知内容，已翻译成渠道的语言。纯文本：请按服务自己的格式进行转义。 |
| `severity` | `info`、`success`、`warning` 或 `error`。未知的值按 `info` 处理。 |

服务接受消息后回复 `{}`。失败时，原因在于配置（令牌已吊销、地址错误）则回复 `-32003` 和 `data.field`，否则回复 `-32000`。Nginx UI 不会重试：这条通知仍然显示在 Nginx UI 中，也仍会通过其他渠道发送。`notify.send` 最多等待 30 秒。

## 校验 {#validating}

```json
{ "jsonrpc": "2.0", "id": 33, "method": "notify.validate", "params": { "channel": "mychat", "config": { "webhook_url": "not a url" } } }
```

在不发送任何内容、不访问服务的情况下检查配置。对第一个缺失或格式错误的字段回复带 `data.field` 的 `-32003`，否则回复 `{}`。创建或修改渠道时 Nginx UI 会调用它，并且不会保存被它拒绝的配置。没有实现这个方法的插件回复 `-32002`，视为没有异议。
