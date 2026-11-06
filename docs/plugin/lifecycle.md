---
outline: [2, 3]
---

# Lifecycle

This page follows a plugin process from start to exit. It applies to plugins
with a `server` block; a plugin without one has no process.

Nginx UI drives the lifecycle with six methods:

| Method | Kind | Purpose |
| --- | --- | --- |
| `plugin.initialize` | request | Handshake: versions, capabilities, settings, permissions. |
| `plugin.initialized` | notification | The handshake succeeded; the plugin may call the host API. |
| `plugin.configure` | request | The settings changed. |
| `plugin.ping` | request | Health check. |
| `plugin.shutdown` | request | Finish the work in progress and prepare to exit. |
| `plugin.exit` | notification | Exit now. |

::: tip
The SDKs implement all of them. You only supply what happens on a settings
change and on shutdown.
:::

## Handshake

Right after starting the process, and before anything else, Nginx UI sends
`plugin.initialize`:

```json
{
  "jsonrpc": "2.0", "id": 1, "method": "plugin.initialize",
  "params": {
    "host": { "version": "2.7.0", "os": "linux", "arch": "amd64", "locale": "en" },
    "settings": {},
    "permissions": ["network"]
  }
}
```

- `settings` is the saved settings of the plugin, `{}` on a first install.
- `permissions` is what the plugin holds. It always matches the manifest,
  since Nginx UI does not start a plugin whose permissions wait for approval.

The plugin answers with what it implements:

```json
{ "jsonrpc": "2.0", "id": 1, "result": { "api_version": 1, "capabilities": ["dns01"] } }
```

| Field | Meaning |
| --- | --- |
| `api_version` | Protocol generation of the process. Must equal the one Nginx UI implements, `1`. |
| `capabilities` | The capabilities the running process implements. Must be exactly the manifest's `capabilities`, as a set. |
| `transports` | Transports the plugin serves, such as `["stdio", "grpc"]`. Standard input and output always work; listing `grpc` enables the [gRPC transport](./protocol.md#grpc-transport). |
| `rpc_socket`, `rpc_pipe`, `rpc_port`, `rpc_token` | Where the gRPC transport listens, see [Protocol](./protocol.md#grpc-transport). |
| `http_pipe`, `http_port` | Where a Windows plugin serves its HTTP endpoints, see [HTTP Endpoints](./http.md#where-to-listen). |

Any other `api_version`, or a capability list that differs from the manifest
in either direction, fails the handshake. Nginx UI waits **10 seconds** for
the answer, so keep startup work (loading a large catalog, warming a cache)
within that or move it after the handshake.

Once the handshake succeeds Nginx UI sends `plugin.initialized`. Only from then
on may the plugin call the [host API](./host-api.md); an earlier call may be
rejected.

## Settings

Whenever a person saves the plugin's settings while it runs, Nginx UI sends
the new settings:

```json
{ "jsonrpc": "2.0", "id": 2, "method": "plugin.configure", "params": { "settings": { "timeout": 90 } } }
```

The plugin answers `{}` once later calls use the new values. The settings of
the handshake are not repeated as a `plugin.configure` call, although
Nginx UI may send one right after the handshake. `host.settings.get` returns
the current settings at any time.

## Health Checks

While the process runs, Nginx UI sends `plugin.ping` every **15 seconds** and
expects `{}` within **5 seconds**. After **3** missed answers in a row it kills
the process and handles it as a crash.

::: warning
A plugin must answer pings even while it is busy with other calls. Handle them
independently of slow work, or at least ahead of it. The SDKs do this for you.
:::

## Stopping

Nginx UI stops a plugin when it is disabled, uninstalled, updated, idle long
enough (for `on_demand` plugins) or when Nginx UI shuts down:

1. It sends `plugin.shutdown` and waits up to **5 seconds** for the answer.
   Use this step to finish the calls in progress, including open gRPC calls
   and log streams, and to refuse new ones. Then answer `{}`.
2. It sends `plugin.exit`, whether or not the answer came.
3. It waits up to **5 seconds** for the process to exit.
4. It kills the process.

On `plugin.exit` a plugin exits promptly, without waiting for more input,
closing its gRPC and HTTP listeners on the way. It exits with status `0`.
Any other status, or an exit without these messages, counts as a crash.

## Crashes and Restarts

A `resident` process that exits on its own has crashed. Nginx UI restarts it
after 1, 2, 4, 8 and then 16 seconds. After 3 crashes within 5 minutes it
stops trying and marks the plugin as failed until someone restarts it.

An `on_demand` process starts when it is first needed and stops after
`idle_timeout_seconds` without calls. The next call starts it again. A crash
while nobody was waiting for it does not count against the limit.

::: warning
A process can also be killed at any moment without the stop sequence, for
example when it exceeds its memory limit. Keep the files in the data
directory consistent without relying on `plugin.shutdown`: write to a
temporary file and rename it, or use a database that survives a crash.
:::

## Environment

Nginx UI sets these variables for the process:

| Variable | Meaning |
| --- | --- |
| `NGINX_UI_PLUGIN_ID` | The plugin id. |
| `NGINX_UI_PLUGIN_API_VERSION` | The protocol generation Nginx UI implements, such as `1`. |
| `NGINX_UI_PLUGIN_DATA_DIR` | Absolute path of a directory only this plugin may write to. Keep all state here. |
| `NGINX_UI_VERSION` | The Nginx UI version. |
| `NGINX_UI_DEMO` | `1` when Nginx UI runs as a public demo, so the plugin can show placeholders instead of data a demo cannot provide. Unset otherwise. |
| `NGINX_UI_PLUGIN_HTTP_SECRET` | The secret of the HTTP endpoints, for a plugin that serves them. See [HTTP Endpoints](./http.md). |

### Outbound Proxy

When Nginx UI is configured with an outbound HTTP proxy, a plugin holding the
`network` permission receives it in `HTTP_PROXY`, `HTTPS_PROXY` and
`NO_PROXY` (and their lowercase forms), so the standard HTTP clients of most
languages use it without extra code. `NO_PROXY` always includes the loopback
addresses. The variables are read when the process starts, so a changed proxy
reaches the plugin on its next start. A plugin without `network` never
receives them.

## Resource Limits

On Linux with cgroup v2, Nginx UI can limit the memory and CPU time of plugin
processes. The operator sets the limits under **Preferences > Plugins**, and a
plugin's [`server.resources`](./manifest.md#resources) hints can lower them
but never raise them. Each plugin runs in its own group,
`<cgroup root>/nginx-ui/plugins/<plugin id>`, with swap disabled.

A process that exceeds its memory limit is killed immediately and handled as
a crash, so a plugin that keeps exceeding it ends up failed. Where limits
cannot be enforced (another operating system, missing privileges, a
container without cgroup delegation) the plugin runs without them, and its
details say the limits are not enforced.

## Conflicting Plugins

Two plugins that [conflict](./manifest.md#dependencies-and-conflicts) never
run at the same time:

- Enabling a plugin while a conflicting one is enabled fails with an error
  that names the other plugin, unless the person chooses to replace it, which
  disables it first.
- A package installed and enabled in one step stays disabled when a
  conflicting plugin is enabled, unless replacing was chosen.
- At startup, a plugin that conflicts with one that stays enabled is disabled
  and records an error naming the other plugin.
