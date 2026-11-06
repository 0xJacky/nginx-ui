---
outline: [2, 3]
---

# Permissions and Security

## What a Plugin Can Do

::: danger
A plugin's server process runs as the same operating system user as Nginx UI.
Nothing sandboxes it: it can read the files that user can read and open
network connections like any other program. Installing a plugin therefore
means trusting whoever published it, which is what
[signatures and trust levels](./signing.md) help people decide.
:::

Permissions are a different thing. They decide which **Nginx UI features** a
plugin may use (its key-value store, stored credentials, notifications, the
REST API of Nginx UI) and which data Nginx UI hands to it (certificates,
access logs). They also tell the person installing the plugin what it is
going to do.

Each plugin gets a data directory of its own, which it alone may write to.
Resource limits, where Nginx UI applies them, bound what a process consumes,
not what it may access.

## Permissions

A plugin lists the permissions it needs in the `permissions` array of its
manifest:

| Permission | What it allows |
| --- | --- |
| `kv` | The key-value store: `host.kv.get`, `set`, `delete` and `list`. |
| `network` | Nothing Nginx UI enforces. It declares that the plugin itself makes outbound network connections, and should be requested by every plugin that talks to a vendor. Required by `security.blocklist` and `upstream.discovery`. |
| `cron` | Registering scheduled calls at runtime with `host.cron.register` and `unregister`. |
| `notify` | Raising notifications in Nginx UI with `host.notify`. |
| `metrics.read` | Reading Nginx UI's metrics with `host.metrics.snapshot`. |
| `core_api` | Calling the Nginx UI REST API from the browser bundle through `registry.coreHttp`. |
| `mcp` | Publishing the plugin's tools to MCP clients. Required by `mcp`. |
| `cert.deploy` | Receiving certificates and their private keys in `deploy.push`. Required by `cert.deploy`. |
| `log.read` | Receiving every nginx access log line in `log.push`. Required by `log.sink`. |
| `log.files` | Listing the nginx log files with `host.logs.list`, receiving the `log.paths_changed` event, and reading those files and their rotated copies. |
| `nginx.snippet` | Keeping nginx configuration snippets with `host.nginx.snippet.*`. Nginx UI tests the configuration and reloads nginx for every change. |
| `nginx.config.read` | Reading the nginx configuration files with `host.nginx.config.list` and `get`. |
| `sites.read` | Listing the sites with `host.sites.list`. |
| `certs.read` | Listing the certificates, without their private keys, with `host.certs.list`. |
| `credentials.read:<kind>` | Reading stored credentials of one kind with `host.credentials.get`, for example `credentials.read:dns`. |

`host.log`, `host.settings.get`, `host.i18n.locale` and `host.activity.set`
need no permission.

Request only what the plugin uses. The linter warns about a permission that
no capability needs, and about a vendor facing capability without `network`.

### Granted and Requested Permissions

The manifest lists what the plugin asks for. The person installing the
plugin approves that list, and only approved permissions count: a call that
needs a permission the plugin does not hold fails with the
[permission denied error](./protocol.md#error-codes) and does nothing.

When an update asks for a permission the person has not approved, Nginx UI
does not grant it silently. The updated plugin stays stopped, and its
capabilities stay unavailable, until someone approves the new list. An update
that only drops permissions needs no approval, and a dropped permission needs
a new approval if a later version asks for it again. A running plugin therefore
always holds every permission its manifest asks for. The handshake tells the
process which permissions it holds, see [Lifecycle](./lifecycle.md#handshake).

### Sensitive Permissions

Some permissions hand data to the plugin that needs particular care, and
Nginx UI says so when it asks for approval:

- **`mcp`** lets every MCP client Nginx UI authorizes run the plugin's tools.
  Treat tool arguments as untrusted input.
- **`cert.deploy`** gives the plugin the private key of every certificate a
  person binds to one of its targets. Treat the key as a credential and never
  keep it after the call.
- **`log.read`** and **`log.files`** give the plugin nginx logs: client
  addresses, every requested URL with its query string, referers and user
  agents. In many jurisdictions these are personal data. Treat every field as
  untrusted input and never write it to your own logs.
- **`nginx.snippet`** lets a plugin change what nginx serves wherever a
  person includes one of its snippets. Nginx UI applies a snippet only while
  the whole configuration stays valid, but the person including it trusts
  the plugin with that part of the configuration. Tell them what a snippet
  does before asking them to include it.
- **`security.blocklist`** and **`upstream.discovery`** let a plugin decide
  what nginx serves. Nginx UI parses every address and port a plugin returns
  and never copies other text into the nginx configuration, but the person
  relying on a blocklist still trusts its plugin not to deny them.

## Declaring Network Hosts

`network_hosts` lists the hosts a plugin intends to contact:

```json [plugin.json]
"network_hosts": ["api.mydns.example"]
```

Nginx UI shows the list to the person installing the plugin. It is
informational: nothing blocks other hosts.

## Explaining Permissions

`permission_reasons` says why the plugin needs a permission, in a sentence a
person can check against what the plugin does:

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

Nginx UI shows the reason under the permission as a note from the author,
wherever it asks the person to grant the permission: the marketplace, the
install dialog and the approval of new permissions. A permission without a
reason, or with an empty one, shows only the description of Nginx UI. An
empty translation shows the English reason.

Each key must be one of `permissions`, and a reason is at most 300
characters. [`nginx-ui plugin lint`](./rules.md#manifest-permission-reasons)
reports a key that is not and a reason that is too long, and Nginx UI
refuses to install such a manifest. Explain the use, not the
permission: Nginx UI already describes what each permission allows.

## Handling Credentials

::: danger
A credential is any value a person enters as a secret (a credential field of
a DNS provider, a `secret` setting, a `secret` configuration field of a
channel, target or backend), anything `host.credentials.get` returns, and the
private key `deploy.push` carries. A credential must never appear in:

- an error message returned to Nginx UI. Name the field instead, with the
  invalid config error and its `field` detail;
- a `host.log` call;
- the plugin's standard error.
:::

Where it helps debugging, log that a value arrived in a masked form, such as
its length or its last four characters, rather than logging nothing. The Go
SDK's `sdk.Redact` and the Rust SDK's `redact` do this.

::: warning
Treat every value of a capability's `config` as secret, not only the fields
marked `secret`: a webhook URL often embeds a token.
:::

A plugin that passes credentials to a library through environment variables
(many DNS libraries read them from the environment) must restore the
previous values right after the call, so two calls with different credentials
never see each other's values, and must never write them anywhere
persistent.

Nginx UI keeps its side of this: it stores secrets encrypted, never sends a
stored secret back to the browser, and never writes secret values to its
logs.
