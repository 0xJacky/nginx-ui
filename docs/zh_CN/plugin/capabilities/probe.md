---
outline: [2, 3]
---

# 健康检查

`probe` 插件在内置的 HTTP 和 gRPC 检查之外，添加检查网站是否健康的方式：TCP 横幅、数据库登录、状态 API。它的类型会作为网站健康检查的选项出现，Nginx UI 按检查的计划运行所选的类型。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `probe.check` | 是 | 对一个目标执行一次检查。 |

`probe` 插件接收调用不需要任何权限，但由于它要访问目标，应请求 `network`。

## 声明类型 {#declaring-kinds}

```json [plugin.json]
"capabilities": ["probe"],
"permissions": ["network"],
"probe": {
  "kinds": [
    {
      "code": "tcp-banner",
      "name": "TCP banner",
      "configuration": {
        "fields": [
          { "key": "port", "type": "number", "display_name": "Port", "required": true },
          { "key": "expect", "display_name": "Expected banner prefix", "help_text": "For example SSH-2.0" }
        ]
      }
    }
  ]
}
```

| 字段 | 必填 | 含义 |
| --- | --- | --- |
| `code` | 是 | 类型的标识，参见[命名规则](../naming.md#provider-and-kind-codes)。在清单内唯一。 |
| `name` | 是 | 类型名称。 |
| `configuration.fields` | 否 | 检查表单的字段，参见[配置表单](./notify.md#configuration-form)。 |

## 检查 {#checking}

```json
{
  "jsonrpc": "2.0", "id": 34, "method": "probe.check",
  "params": { "kind": "tcp-banner", "target": "https://example.com", "config": { "port": "22", "expect": "SSH-2.0" }, "timeout_seconds": 10 }
}
```

| 字段 | 含义 |
| --- | --- |
| `kind` | 类型的 `code`。 |
| `target` | 检查的对象：网站地址，或健康检查的自定义目标。类型只使用它需要的部分，例如 TCP 检查只用主机名。 |
| `config` | 检查表单的值。请当作机密处理。 |
| `timeout_seconds` | Nginx UI 等待的时长。请在此时间内完成。 |

```json
{ "jsonrpc": "2.0", "id": 34, "result": { "status": "down", "latency_ms": 10000, "message": "no banner within 10s" } }
```

| 字段 | 含义 |
| --- | --- |
| `status` | `up`、`down` 或 `degraded`。 |
| `latency_ms` | 检查耗时，不能为负数。 |
| `message` | 显示给用户的详情，`status` 不是 `up` 时应当提供。绝不能包含凭据。 |

`degraded` 表示目标有响应，但状况不如预期：响应慢、部分失败或者给出了警告。目标宕机或没有及时响应是一个**结果**：请报告 `down` 并附上消息。只有检查本身无法运行时才回复错误：配置无法使用时回复带 `data.field` 的 `-32003`，其他情况回复 `-32000`。

Nginx UI 会把调用失败（错误、超时、插件不可用）与 `down` 区分显示，使用户能分辨是目标宕机还是检查没有运行。它把 `degraded` 视为在线并附带消息，并在 `timeout_seconds` 之外再多等待五秒。
