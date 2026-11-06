---
outline: [2, 3]
---

# Plugin Development Overview

Plugins extend Nginx UI without changing Nginx UI itself. A plugin can add a
DNS provider for certificate issuance, a notification channel, a backup
destination, pages and panels in the web interface, nginx configuration
templates and much more. Plugins are installed from a catalog, from an
uploaded package or from a directory on the server, and can be enabled,
disabled and updated at any time.

This guide is for people who write plugins. It describes everything a plugin
must provide and everything it can rely on, so a plugin can be written in any
language.

## What a Plugin Is Made Of

A plugin is a package with a `plugin.json` [manifest](./manifest.md) at its
root. The manifest declares one or more of three parts:

| Part | What it is | When to use it |
| --- | --- | --- |
| `server` | An executable that Nginx UI starts and talks to. | The plugin runs code on the server: it calls a vendor API, serves HTTP, answers capability calls. |
| `webapp` | A JavaScript bundle loaded into the Nginx UI web interface, or static HTML pages shown in a frame. | The plugin adds pages, panels, form fields or columns to the interface. |
| `content` | nginx configuration templates and translation files. | The plugin only contributes data. Such a plugin has no process at all. |

A plugin can combine the three. The official DNS-01 plugin, for example, ships
a server process that talks to DNS providers and a browser bundle that adds
its fields to the certificate form.

## How the Server Process Works

Nginx UI starts the executable and exchanges
[JSON-RPC 2.0](https://www.jsonrpc.org/specification) messages with it over
the process's standard input and output, one JSON object per line. Both sides
can send requests:

- Nginx UI calls the plugin to run its [lifecycle](./lifecycle.md) (start,
  configure, health checks, stop) and to use its
  [capabilities](#capabilities).
- The plugin calls Nginx UI through the [host API](./host-api.md): a key-value
  store, its settings, notifications, scheduled tasks, stored credentials.

A plugin may additionally serve the same calls over gRPC for speed, and must
do so for access log streaming. The [protocol](./protocol.md) page covers both
transports.

The process runs with the same operating system privileges as Nginx UI.
[Permissions](./permissions.md) decide which Nginx UI features it can use,
not what it can do on the machine, so installing a plugin means trusting its
publisher. [Signatures](./signing.md) tell the person installing a plugin who
published it.

## Capabilities

A capability is a feature Nginx UI offers next to its own and hands to a
plugin. A plugin declares the capabilities it implements in its manifest, and
Nginx UI calls it whenever that feature is used.

| Capability | What the plugin does | Page |
| --- | --- | --- |
| `dns01` | Publishes the TXT records of the ACME DNS-01 challenge through a DNS provider. | [DNS-01](./capabilities/dns01.md) |
| `http` | Serves an HTTP API or pages behind Nginx UI's authentication. | [HTTP Endpoints](./http.md) |
| `notify` | Delivers Nginx UI notifications through a chat service, push gateway or paging system. | [Notification Channels](./capabilities/notify.md) |
| `probe` | Adds kinds of site health checks. | [Health Checks](./capabilities/probe.md) |
| `mcp` | Adds tools to the Nginx UI MCP server for AI assistants. | [MCP Tools](./capabilities/mcp.md) |
| `storage` | Stores backups in places Nginx UI cannot reach on its own. | [Storage](./capabilities/storage.md) |
| `cert.deploy` | Pushes issued certificates to CDNs, load balancers and other servers. | [Certificate Deployment](./capabilities/cert-deploy.md) |
| `security.blocklist` | Fetches lists of addresses to deny. | [Blocklists](./capabilities/blocklist.md) |
| `upstream.discovery` | Resolves a service into the servers of an nginx upstream. | [Upstream Discovery](./capabilities/discovery.md) |
| `log.sink` | Receives the nginx access log lines as nginx writes them. | [Access Log Streaming](./capabilities/log-sink.md) |

Templates and translation files need no capability: they are
[content](./capabilities/content.md).

## SDKs

The protocol is plain JSON over pipes, so any language works. SDKs take care
of the protocol for you:

| SDK | Language | Covers |
| --- | --- | --- |
| [plugin-sdk-go](https://github.com/nginxui/plugin-sdk-go) | Go | The server process: protocol, lifecycle, every capability, gRPC, HTTP. |
| [plugin-sdk-rust](https://github.com/nginxui/plugin-sdk-rust) | Rust | The server process: protocol, lifecycle, every capability, gRPC, HTTP. |
| [@nginxui/plugin-sdk](https://github.com/nginxui/plugin-sdk-web) | TypeScript | The browser bundle: types of the runtime, a Vite preset, helpers for static pages. |

Plugins in other languages speak the protocol directly. The
[plugin-spec](https://github.com/nginxui/plugin-spec) repository holds the
machine-readable contract every SDK is built from: the protobuf definitions of
every message, JSON Schemas of the manifest and the catalog, request and
response samples, and a Python plugin with no dependencies.

## Where to Go Next

- [Quick Start](./quick-start.md) creates, builds and installs a first plugin.
- [Develop and Debug](./development.md) explains the tools for checking a
  plugin before you publish it.
- [Packaging](./packaging.md), [Signing and Trust](./signing.md) and
  [Catalogs](./catalog.md) cover publishing.
