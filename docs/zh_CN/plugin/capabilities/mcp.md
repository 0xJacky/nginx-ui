---
outline: [2, 3]
---

# MCP 工具

`mcp` 插件向 Nginx UI 的 [MCP 服务器](../../guide/mcp.md)添加工具，使连接到 Nginx UI 的 AI 助手可以在内置工具之外调用它们。Nginx UI 以插件 ID 派生的名称发布每个工具，像对待自己的工具一样对每次调用进行授权，然后把调用转发给插件。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `mcp.call` | 是 | 运行一个工具。 |

## 声明工具 {#declaring-tools}

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

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `name` | 是 | 插件内的工具名称，匹配 `^[a-z0-9][a-z0-9_-]{0,47}$`，唯一。 |
| `description` | 是 | 工具的作用，供助手决定是否使用。 |
| `input_schema` | 否 | 参数的 JSON Schema，其 `type` 为 `object`。没有时工具不接受参数。 |

必须请求 `mcp` 权限，它会告诉批准插件的人：AI 助手将能够运行插件的代码。`io.github.example.cdn` 的 `purge_cache` 工具发布为 `io_github_example_cdn__purge_cache`，参见[命名规则](../naming.md#mcp-tool-names)。

## 运行工具 {#running-a-tool}

```json
{
  "jsonrpc": "2.0", "id": 37, "method": "mcp.call",
  "params": { "tool": "purge_cache", "arguments": { "zone": "example.com", "paths": ["/index.html"] } }
}
```

`tool` 是清单中的名称，不带发布时的前缀；`arguments` 是客户端发送的内容。**请先校验参数再使用**：参数来自 AI 助手，可能不符合 Schema。

```json
{ "jsonrpc": "2.0", "id": 37, "result": { "content": [{ "type": "text", "text": "Purged 1 path in zone example.com." }] } }
```

| 字段 | 含义 |
| --- | --- |
| `content` | 返回给客户端的内容块，按顺序排列。唯一的类型是 `text`，带有 `text`。 |
| `is_error` | 工具运行了但失败了，`content` 说明原因。 |

工具运行后失败（服务拒绝、区域不存在）时，回复一个 `is_error: true` 的结果，使助手读到解释并能自行纠正。只有调用无法处理时才回复协议错误：清单未声明的工具或参数不是对象时回复 `-32602`，其他情况回复 `-32000`。`content` 绝不能包含凭据。

## Nginx UI 如何发布工具 {#how-nginx-ui-publishes-tools}

- 只有在插件已启用并持有 `mcp` 权限时才发布它的工具；插件被禁用、卸载或正在等待新权限批准时撤回。已连接的客户端会收到工具列表变化的通知。
- 每个插件工具都被视为会改变状态的工具：调用需要访问令牌的 `mcp:write` 范围，或已登录用户的安全会话。
- 错误、超时和插件不可用会以带 `isError` 的工具结果返回给客户端。`mcp.call` 最多等待 60 秒。
