---
outline: [2, 3]
---

# Versions and Compatibility

Three version numbers appear in a plugin, and they mean different things:

| Version | Where | What it tracks |
| --- | --- | --- |
| `api_version` | `plugin.json` and the handshake | The generation of the plugin protocol. `1` today. |
| `version` | `plugin.json` | The plugin's own release, in semantic versioning. |
| `min_nginx_ui_version` | `plugin.json` | The oldest Nginx UI release the plugin supports. |

## The Protocol Generation

`api_version` changes only with a breaking change of the protocol: a removed
field, a renamed key, an error code that means something else. Within one
generation the protocol only grows: new optional fields, new capabilities, new
events, new host methods, new slots. Two rules keep that working in both
directions:

- **Ignore what you do not know.** A plugin ignores JSON members, event types
  and slot names it does not recognize, so a newer Nginx UI does not break it.
- **Do not require what is optional.** A plugin treats every optional field as
  possibly absent, so an older Nginx UI that does not send it still works.

The `api_version` of a plugin must equal the one Nginx UI implements. There is
no negotiation: a mismatch fails the handshake with a clear error. When a
second generation exists, both will be documented side by side until the
first is retired, with advance notice.

## The Plugin Version

`version` is yours. Use semantic versioning: raise the major version for
changes that need action from the people using the plugin, the minor version
for new features and the patch version for fixes. A prerelease such as
`1.2.0-beta.1` puts a release on a less stable
[release channel](./catalog.md#release-channels).

## The Minimum Nginx UI Version

`min_nginx_ui_version` names the oldest Nginx UI release a plugin supports,
for example because it uses a host method or a slot added in that release:

```json [plugin.json]
"min_nginx_ui_version": "2.7.0"
```

It is advisory. Nginx UI shows it and may refuse to enable a plugin that
needs a newer release, but it is not part of the protocol, and a plugin should
still check for optional features it uses. In a browser bundle, feature-detect
optional members of the runtime, such as `registry.loadChunk`, rather than
relying on a version.
