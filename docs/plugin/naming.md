---
outline: [2, 3]
---

# Naming

Several names a plugin chooses are shared with every other plugin, so they
follow the rules on this page. Once shipped, none of them should change.

## Plugin Ids

A plugin id is a reversed domain: two or more dot separated segments of
lowercase letters, digits and hyphens, at most 64 characters, most
significant segment first:

```text
com.example.mydns
io.github.example.mydns
```

- Use a domain you control, reversed.
- Without a domain of your own, use `io.github.<owner>.<name>`, where
  `<owner>` is your GitHub user or organization in lowercase. Every GitHub
  user already controls that namespace.
- `com.nginxui.*` is reserved for plugins of the Nginx UI project. The
  official catalog rejects it for anyone else, and Nginx UI installs such an
  id only from a package signed with the official key, whichever catalog or
  file it comes from. Developer mode lifts this, for building the official
  plugins.

::: warning
The id identifies the plugin for good: its settings, its data, its
certificates and its catalog entry all hang off it. Renaming means
publishing a new plugin.
:::

## Provider and Kind Codes

Capabilities that add choices to an Nginx UI form identify each choice with a
`code`:

| Capability | Code of |
| --- | --- |
| `dns01` | a DNS provider |
| `notify` | a notification channel |
| `probe` | a health check kind |
| `storage` | a storage backend |
| `cert.deploy` | a deploy target kind |
| `security.blocklist` | a blocklist source kind |
| `upstream.discovery` | a discovery provider |

A code matches `^[a-z0-9-]{2,32}$` and appears once in a manifest. Codes are
shared across every installed plugin of the same capability: a certificate, a
notification channel or a backup task stores the code alone, and Nginx UI
finds the plugin that owns it when it is used. Therefore:

- **Name the vendor or the technique, not the plugin**: `cloudflare`, not
  `mydns-cloudflare`. Never prefix a code with the plugin id.
- **Avoid codes other plugins already use**, in particular the providers of
  the official DNS-01 plugin, unless your plugin is meant to replace that
  implementation. Nginx UI may refuse to enable a plugin whose code another
  enabled plugin already registered.
- **Never rename a shipped code.** Everything configured with the old code
  stops working.

When several enabled plugins declare the same code, Nginx UI uses the one
with the lowest plugin id. Plugin codes never collide with Nginx UI's own
channels, checks or storage, which are kept apart.

## MCP Tool Names

An MCP tool name matches `^[a-z0-9][a-z0-9_-]{0,47}$` and is unique within
the plugin. Nginx UI publishes it with the plugin id in front, the dots
replaced by underscores, and two underscores in between:

```text
io.github.example.cdn + purge_cache  →  io_github_example_cdn__purge_cache
```

Some model APIs accept only 64 characters, so keep the published name within
64. Do not rename a shipped tool, since clients and prompts refer to it by
name.

## Settings Keys and Slot Names

Settings keys belong to one plugin, so short keys such as `timeout` are fine.

A slot a plugin defines for itself (one that is not one of the
[Nginx UI slots](./slots.md)) should carry the plugin id as a prefix, such as
`io.github.example.mydns:extra-panel`, so it never collides with a future
slot of Nginx UI or of another plugin.

## Platform Keys

A platform key is `<goos>-<goarch>`: the `GOOS` and `GOARCH` values of the Go
toolchain joined with a hyphen, as `go tool dist list` prints them:
`linux-amd64`, `linux-arm64`, `darwin-arm64`, `windows-amd64` and so on. They
are used in `server.executables`, in the `platforms` and `downloads` of a
catalog release and in package file names. `any` stands for every platform in
a catalog, never in a file name.

## Package File Names

```text
<id>-<version>.tar.gz                  portable package
<id>-<version>-<goos>-<goarch>.tar.gz  package for one platform
```

For example `com.nginxui.dns01-1.0.0.tar.gz` and
`com.nginxui.dns01-1.0.0-linux-arm64.tar.gz`. A file name is read as
per-platform only when its last two hyphen separated parts are a known GOOS
and GOARCH, so do not publish a version whose prerelease part ends in such a
pair, such as `1.0.0-linux-amd64`.

Nginx UI never relies on the file name: everything it needs comes from the
manifest inside, and an uploaded package may carry any name.
