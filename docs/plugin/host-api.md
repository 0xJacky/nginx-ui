---
outline: [2, 3]
---

# Host API

After the handshake a plugin can call Nginx UI through `host.*` methods. They
are ordinary JSON-RPC requests from the plugin to Nginx UI, over standard input
and output. Nginx UI also calls the plugin on its own, for subscribed events
and scheduled tasks.

| Method | Permission | Purpose |
| --- | --- | --- |
| `host.log` | none | Write a structured log line. |
| `host.kv.get`, `set`, `delete`, `list` | `kv` | Use the plugin's key-value store. |
| `host.settings.get` | none | Read the plugin's current settings. |
| `host.i18n.locale` | none | Read the interface language. |
| `host.credentials.get` | `credentials.read:<kind>` | Read a stored credential. |
| `host.cron.register`, `unregister` | `cron` | Schedule calls at runtime. |
| `host.notify` | `notify` | Raise a notification in Nginx UI. |
| `host.metrics.snapshot` | `metrics.read` | Read Nginx UI's metrics. |
| `host.logs.list` | `log.files` | List the nginx log files the plugin may read. |
| `host.activity.set` | none | Show background work in the processing indicator. |
| `host.nginx.snippet.put`, `delete`, `list` | `nginx.snippet` | Keep nginx configuration snippets. |
| `host.nginx.config.list`, `get` | `nginx.config.read` | Read the nginx configuration files. |
| `host.sites.list` | `sites.read` | List the sites. |
| `host.certs.list` | `certs.read` | List the certificates. |

::: info
Calls are allowed only after `plugin.initialized` and only with the
permissions the plugin actually holds (see
[Permissions and Security](./permissions.md#granted-and-requested-permissions)).
A call without the permission fails with `-32001` and does nothing.
:::

## Logging

```json
{ "jsonrpc": "2.0", "id": 20, "method": "host.log", "params": { "level": "info", "message": "reconfigured", "fields": { "settings_count": 2 } } }
```

`level` is `debug`, `info`, `warn` or `error`, and `fields` is optional
structured context. Nginx UI answers `{}` and shows the line with the
plugin's other log output. Lines written to standard error appear there too.
Never log a credential.

## Key-Value Store

Each plugin has a key-value store of its own. Keys are non-empty strings and
values any JSON value of at most **64 KiB** encoded.

::: code-group

```json [Request]
{ "jsonrpc": "2.0", "id": 21, "method": "host.kv.set", "params": { "key": "last_sync", "value": { "at": 1732000000 } } }
{ "jsonrpc": "2.0", "id": 22, "method": "host.kv.get", "params": { "key": "last_sync" } }
```

```json [Response]
{ "jsonrpc": "2.0", "id": 22, "result": { "value": { "at": 1732000000 }, "found": true } }
```
:::

| Method | Params | Result |
| --- | --- | --- |
| `host.kv.get` | `key` | `value` and `found`. A missing key is `found: false`, not an error. |
| `host.kv.set` | `key`, `value` | `{}`. A larger value fails with `-32602`. |
| `host.kv.delete` | `key` | `{}`, whether or not the key existed. |
| `host.kv.list` | optional `prefix` | `keys`, an array, empty when nothing matches. |

Two plugins never see each other's keys. For larger or structured data, use
files in the plugin's data directory instead.

## Settings and Language

`host.settings.get` returns the settings of the latest handshake or
`plugin.configure`, so a plugin need not keep its own copy:

```json
{ "jsonrpc": "2.0", "id": 23, "result": { "settings": { "timeout": 120 } } }
```

`host.i18n.locale` returns the interface language, such as `{ "locale":
"zh_CN" }`, for localizing notifications or log messages.

## Credentials

`host.credentials.get` reads a credential stored in Nginx UI by its kind and
id:

::: code-group

```json [Request]
{ "jsonrpc": "2.0", "id": 25, "method": "host.credentials.get", "params": { "kind": "dns", "id": "7" } }
```

```json [Response]
{ "jsonrpc": "2.0", "id": 25, "result": { "id": "7", "name": "Cloudflare - example.com", "provider_code": "cloudflare", "config": { "CF_DNS_API_TOKEN": "..." } } }
```
:::

The permission names the kind, such as `credentials.read:dns`, and covers that
kind only. A DNS-01 plugin rarely needs this method: the credential of a
certificate already arrives in each call.

## Scheduled Tasks

A plugin can have Nginx UI call one of its methods on a schedule. Declare
fixed schedules in the manifest, which needs no permission:

```json [plugin.json]
"cron": [
  { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" }
]
```

or register them at runtime with the `cron` permission:

```json
{ "jsonrpc": "2.0", "id": 26, "method": "host.cron.register", "params": { "id": "refresh-catalog", "schedule": "@every 24h", "method": "catalog.refresh" } }
```

- `schedule` is a five field cron expression or `@every <duration>`.
- `method` is a method the plugin serves, preferably a name of its own such
  as `catalog.refresh`.
- Registering an `id` again replaces the previous entry.
  `host.cron.unregister` with `{ "id": ... }` removes it and answers `{}` even
  when it did not exist.

When a schedule fires, Nginx UI sends an ordinary request to that method:

```json
{ "jsonrpc": "2.0", "id": 31, "method": "catalog.refresh", "params": { "type": "refresh-catalog", "ts": 1732000000 } }
```

`type` is the entry id and `ts` the Unix time it fired. Answer the request;
the result is ignored and an error is logged without a retry. Nginx UI allows
**10 minutes** per call, starts an `on_demand` plugin for it and never starts
an entry again while its previous call still runs.

## Notifications

```json
{ "jsonrpc": "2.0", "id": 27, "method": "host.notify", "params": { "level": "warning", "title": "Propagation slow", "content": "example.com is taking longer than usual to propagate.", "details": { "domain": "example.com" } } }
```

`level` is `info`, `success`, `warning` or `error`. The notification appears
among the notifications of Nginx UI, and `details` is optional context shown
on demand. Nginx UI answers `{}` once it queued the
notification.

The `notify` permission is unrelated to the `notify` capability, which lets a
plugin deliver notifications, see
[Notification Channels](./capabilities/notify.md).

## Metrics

`host.metrics.snapshot` returns Nginx UI's current metrics, such as
`{ "snapshot": { "cpu_percent": 3.2, "memory_bytes": 104857600 } }`. The shape
is not guaranteed across versions: treat it as best-effort telemetry.

## Log Files

With the `log.files` permission a plugin can read the nginx log files itself.
`host.logs.list` returns the files Nginx UI allows reading:

```json
{ "jsonrpc": "2.0", "id": 29, "result": { "logs": [
  { "path": "/var/log/nginx/access.log", "type": "access", "source": "default" },
  { "path": "/var/log/nginx/example.com.error.log", "type": "error", "source": "config", "config_file": "/etc/nginx/sites-enabled/example.com.conf" }
] } }
```

| Field | Meaning |
| --- | --- |
| `path` | Absolute path of the live log file. |
| `type` | `access` or `error`. |
| `source` | `config` when an nginx configuration names the path, `default` for nginx's built-in default logs. |
| `config_file` | The configuration file that names the path. |

Rotated files (`access.log.1`, `access.log.2.gz`, `access.log-20260101`) are
not listed; look for them next to a listed path. Read only listed files and
their rotated copies, treat their content as personal data, and never write
it to your own logs. The list changes when the nginx configuration changes:
subscribe to `log.paths_changed` and call `host.logs.list` again when it
arrives.

To receive new lines as nginx writes them instead of reading files, see
[Access Log Streaming](./capabilities/log-sink.md).

## nginx Configuration

### Snippets

With the `nginx.snippet` permission a plugin keeps nginx configuration of its
own, such as cache rules or a rate limit. `host.nginx.snippet.put` writes one
snippet:

::: code-group

```json [Request]
{ "jsonrpc": "2.0", "id": 31, "method": "host.nginx.snippet.put", "params": { "name": "static-cache", "content": "location ~* \\.(css|js)$ {\n  expires 7d;\n}\n" } }
```

```json [Response]
{ "jsonrpc": "2.0", "id": 31, "result": { "changed": true, "include": "include snippets/plugins/io.github.example.cache/static-cache.conf;" } }
```

:::

- `name` is 1 to 64 characters of `[a-z0-9_-]` starting with a letter or
  digit, and `content` is at most 256 KiB of UTF-8. A plugin keeps at most 32
  snippets.
- Nginx UI writes the snippet, tests the whole configuration and reloads
  nginx. When nginx rejects the configuration, the previous snippet stays,
  nothing is reloaded and the call fails with `-32602` and what nginx said.
- `changed` is `false` when the snippet already had this content, and
  nothing is reloaded then.

A snippet takes effect only where it is included. Show `include` to the
person, for example in your settings panel, and let them add it to the
`server` or `location` block where it belongs. Nginx UI never adds it on its
own.

`host.nginx.snippet.list` returns
`{ "snippets": [{ "name": "...", "include": "..." }] }`, and
`host.nginx.snippet.delete` with `{ "name": "..." }` removes one snippet and
answers `{ "removed": true }`. A snippet that is still included cannot go:
nginx would reject the configuration, so the snippet stays and the call fails
with `-32602`.

::: info
Snippets stay when the plugin is disabled, so nginx keeps serving what it
served. When the plugin is uninstalled, Nginx UI removes its snippets, and
empties a snippet that is still included so the configuration stays valid.
:::

### Reading the Configuration

With `nginx.config.read`, `host.nginx.config.list` returns the configuration
files relative to the nginx configuration directory, such as
`["conf.d/gzip.conf", "nginx.conf", "sites-available/example.com"]`, and
`host.nginx.config.get` with `{ "path": "nginx.conf" }` returns
`{ "content": "..." }`. The list holds `nginx.conf`, every `.conf` file and
the files in `sites-available` and `streams-available`. Keys, password files
and symbolic links are left out, and a file is at most 1 MiB.

## Sites and Certificates

`host.sites.list` (`sites.read`) returns the sites:

```json
{ "sites": [ { "name": "example.com", "status": "enabled", "urls": ["https://example.com"], "config_file": "sites-available/example.com" } ] }
```

`status` is `enabled`, `disabled` or `maintenance`, and `config_file` can be
read with `host.nginx.config.get`.

`host.certs.list` (`certs.read`) returns the certificates, never with their
private keys:

```json
{ "certs": [ { "id": "3", "name": "example.com", "domains": ["example.com", "www.example.com"], "auto_renew": true, "challenge_method": "dns01", "key_type": "P256", "not_before": "2026-09-01T00:00:00Z", "not_after": "2026-11-30T00:00:00Z", "issuer": "Let's Encrypt" } ] }
```

The dates and the issuer are empty when the certificate file cannot be read.
Subscribe to the `site.*` and `cert.*` [events](#events) to know when to list
again.

## Processing Indicator

`host.activity.set` shows the plugin's background work in the processing
indicator of Nginx UI, next to its own tasks:

```json
{ "jsonrpc": "2.0", "id": 30, "method": "host.activity.set", "params": { "key": "indexing", "label": "Indexing access logs", "active": true } }
```

- `key` identifies the entry within the plugin: 1 to 64 characters of
  `[a-z0-9._-]`.
- `label` is an English text of 1 to 128 characters. Nginx UI translates it
  with the translations the plugin's browser bundle registers, keyed by the
  same English text.
- `active: true` shows or updates the entry; `active: false` removes it.

Nginx UI allows at least 8 entries per plugin and removes all of them when
the plugin stops, crashes or is disabled.

## Events

A plugin subscribes to Nginx UI events in the `events` array of its manifest:

```json [plugin.json]
"events": ["cert.renewed", "nginx.reload_failed"]
```

Nginx UI delivers each event as an `events.on` notification, which is never
answered:

```json
{ "jsonrpc": "2.0", "method": "events.on", "params": { "type": "cert.renewed", "data": { "domain": "example.com" }, "ts": 1732000000 } }
```

| Event | When |
| --- | --- |
| `cert.issued` | A certificate finished issuing. |
| `cert.renewed` | A certificate was renewed. |
| `cert.expiring` | A certificate is close to expiry. |
| `site.saved` | A site configuration was saved. |
| `site.enabled` | A site was enabled. |
| `site.disabled` | A site was disabled. |
| `nginx.reloaded` | nginx reloaded. |
| `nginx.reload_failed` | An nginx reload failed. |
| `node.status_changed` | A cluster node's status changed. |
| `node.joined` | A node joined the cluster. |
| `backup.completed` | A backup finished. |
| `auth.login_failed` | A login attempt failed. |
| `plugin.changed` | A plugin was installed, updated, enabled or disabled. |
| `log.paths_changed` | The files `host.logs.list` returns changed. Delivered only to a plugin holding `log.files`. |

Only subscribed events are delivered. Ignore an event type you do not know:
new ones may be added.
