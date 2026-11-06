---
outline: [2, 3]
---

# Notification Channels

A `notify` plugin delivers Nginx UI notifications through a service of its
own: a chat service, a push gateway, a paging system. Its channels appear next
to the built-in notification channels. A person configures a channel once,
and Nginx UI calls the plugin whenever a notification is routed to it.

| Method | Required | Purpose |
| --- | --- | --- |
| `notify.send` | yes | Deliver one notification. |
| `notify.validate` | no | Check a channel configuration without sending anything. |

The `notify` capability is not the `notify` permission: the permission lets a
plugin raise a notification in Nginx UI with `host.notify`. A `notify` plugin
needs no permission to receive `notify.send`, but should request `network`
since it talks to a service.

## Declaring Channels

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

| Field | Required | Meaning |
| --- | --- | --- |
| `code` | yes | Identifier of the channel, see [Naming](../naming.md#provider-and-kind-codes). Unique in the manifest. |
| `name` | yes | Name of the channel. |
| `configuration.fields` | no | The fields of the channel form. |

## Configuration Form

Channels, health check kinds, storage backends, deploy targets, blocklist
sources and discovery providers all describe their form with the same
`configuration.fields`:

| Field | Required | Meaning |
| --- | --- | --- |
| `key` | yes | Key of the value in `config`. Not empty, unique in the form. |
| `type` | no | `text` (default), `textarea`, `number` or `bool`. |
| `display_name` | yes | Field label. |
| `help_text` | no | Description under the field. |
| `required` | no | Nginx UI does not save a configuration that leaves the field empty. |
| `secret` | no | A credential: shown masked and never logged. |

Every value reaches the plugin as a string in `config`: a number as decimal
text such as `"30"`, a `bool` as `"true"` or `"false"`. An empty field may be
absent. Nginx UI stores configurations encrypted.

::: warning
Treat **every** `config` value as secret, not only `secret` fields: a webhook
URL often embeds a token.
:::

## Sending

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

| Field | Meaning |
| --- | --- |
| `channel` | The `code` of the channel. |
| `config` | The values of the channel form. |
| `title`, `content` | The notification, already translated into the channel's language. Plain text: escape it for the service's own format. |
| `severity` | `info`, `success`, `warning` or `error`. Treat unknown values as `info`. |

Answer `{}` once the service accepted the message. On failure answer
`-32003` with `data.field` when the cause is the configuration (a revoked
token, a wrong URL) and `-32000` otherwise. Nginx UI does not retry: the
notification stays visible in Nginx UI and still goes out through the other
channels. It waits 30 seconds for `notify.send`.

## Validating

```json
{ "jsonrpc": "2.0", "id": 33, "method": "notify.validate", "params": { "channel": "mychat", "config": { "webhook_url": "not a url" } } }
```

Check the configuration without sending anything or contacting the service.
Answer `-32003` with `data.field` for the first missing or malformed field
and `{}` otherwise. Nginx UI calls it when a channel is created or changed
and does not save a configuration it rejects. A plugin without this method
answers `-32002`, which counts as no objection.
