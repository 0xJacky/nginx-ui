---
outline: [2, 3]
---

# MCP Tools

An `mcp` plugin adds tools to the [MCP server](../../guide/mcp.md) of
Nginx UI, so an AI assistant connected to Nginx UI can call them next to the
built-in tools. Nginx UI publishes each tool under a name derived from the
plugin id, authorizes every call like a call to its own tools, and forwards it
to the plugin.

| Method | Required | Purpose |
| --- | --- | --- |
| `mcp.call` | yes | Run one tool. |

## Declaring Tools

```json [plugin.json]
"capabilities": ["mcp"],
"permissions": ["mcp", "network"],
"mcp": {
  "tools": [
    {
      "name": "purge_cache",
      "description": "Purge cached paths of a CDN zone.",
      "input_schema": {
        "type": "object",
        "properties": {
          "zone": { "type": "string", "description": "Zone name, e.g. example.com" },
          "paths": { "type": "array", "items": { "type": "string" } }
        },
        "required": ["zone"]
      }
    }
  ]
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Tool name within the plugin, matching `^[a-z0-9][a-z0-9_-]{0,47}$`. Unique. |
| `description` | yes | What the tool does, for the assistant deciding whether to use it. |
| `input_schema` | no | JSON Schema of the arguments. Its `type` is `object`. Without it the tool takes no arguments. |

The `mcp` permission is required. It tells the person approving the plugin
that an AI assistant will be able to run the plugin's code. The tool
`purge_cache` of `io.github.example.cdn` is published as
`io_github_example_cdn__purge_cache`, see [Naming](../naming.md#mcp-tool-names).

## Running a Tool

```json
{
  "jsonrpc": "2.0", "id": 37, "method": "mcp.call",
  "params": { "tool": "purge_cache", "arguments": { "zone": "example.com", "paths": ["/index.html"] } }
}
```

`tool` is the name from the manifest, without the published prefix, and
`arguments` what the client sent. **Validate the arguments before acting on
them**: they come from an AI assistant and may not match the schema.

```json
{ "jsonrpc": "2.0", "id": 37, "result": { "content": [{ "type": "text", "text": "Purged 1 path in zone example.com." }] } }
```

| Field | Meaning |
| --- | --- |
| `content` | Content blocks for the client, in order. The only type is `text`, with `text`. |
| `is_error` | The tool ran and failed; `content` explains why. |

When the tool runs and fails (the service refused, the zone does not exist),
answer a result with `is_error: true`, so the assistant reads the explanation
and can correct itself. Answer a protocol error only when the call cannot be
handled: `-32602` for a tool the manifest does not declare or arguments that
are not an object, `-32000` otherwise. `content` never contains a credential.

## How Nginx UI Publishes Tools

- Tools are published only while the plugin is enabled and holds the `mcp`
  permission, and withdrawn when it is disabled, uninstalled or waiting for
  approval of new permissions. Connected clients are told that the tool list
  changed.
- Every plugin tool is treated as a tool that changes state: a call needs the
  `mcp:write` scope of an access token, or a secure session of a signed in
  person.
- Errors, timeouts and an unavailable plugin reach the client as a tool
  result with `isError`. Nginx UI waits 60 seconds for `mcp.call`.
