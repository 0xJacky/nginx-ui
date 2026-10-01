---
outline: [2, 3]
---

# 通信协议

Nginx UI 与插件进程通过进程的标准输入和标准输出交换 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) 消息。双方都会发送请求：Nginx UI 调用生命周期和能力方法，插件调用宿主 API。插件还可以通过 gRPC 提供能力调用，这在负载较大时更快，而访问日志推送必须使用 gRPC。

::: tip 提示
SDK 实现了本页的全部内容。不使用 SDK 编写插件，或者想了解底层细节时，请阅读本页。
:::

## 消息帧 {#framing}

- Nginx UI 写入插件的标准输入，插件写入自己的标准输出。
- 每条消息是一行上的一个 JSON 对象，后跟一个换行符。消息内部永远不含原始换行符。
- **标准输出只能用于协议消息。** 日志、横幅和堆栈跟踪都写到标准错误，Nginx UI 会把它收集为插件日志，但从不解析它。
- 不支持批量消息（JSON 数组）。
- 一条消息包括换行符最多 **4 MiB**，更大的消息会终止连接。

## 消息 {#messages}

每条消息都包含 `"jsonrpc": "2.0"`，其他成员决定它的类型：

| 类型 | `id` | `method` | `params` | `result` | `error` |
| --- | --- | --- | --- | --- | --- |
| 请求 | 存在且不为 `null` | 存在 | 可选 | | |
| 通知 | 不存在 | 存在 | 可选 | | |
| 成功响应 | 请求的 `id` | | | 存在 | |
| 错误响应 | 请求的 `id` | | | | 存在 |

- 每个请求恰好得到一个响应，通知永远不会得到响应。
- `id` 是用于把响应和请求对应起来的不透明值，SDK 使用小整数。
- `params` 总是一个对象。

请求可以并发执行，并以任意顺序得到响应，所以请用 `id` 匹配响应。插件可以并发处理请求，也可以逐个处理，但忙碌时仍必须回应健康检查（参见[生命周期](./lifecycle.md#health-checks)）。

## 错误 {#errors}

```json
{ "jsonrpc": "2.0", "id": 7, "error": { "code": -32003, "message": "the API token is required", "data": { "field": "MYDNS_API_TOKEN" } } }
```

`message` 简短易读，绝不能包含凭据。`data` 是可选的对象，包含详细信息。

### 错误码 {#error-codes}

| 代码 | 名称 | 使用场景 |
| --- | --- | --- |
| `-32700` | 解析错误 | 消息不是有效的 JSON。 |
| `-32600` | 无效请求 | 消息是 JSON，但不是有效的 JSON-RPC 消息。 |
| `-32601` | 方法不存在 | 该方法没有处理程序。 |
| `-32602` | 参数无效 | `params` 的结构不符合预期，或者在能力页面说明的情况下引用了方法不认识的内容。 |
| `-32000` | 内部错误 | 调用已执行，但因其他原因失败：服务商故障、网络故障。 |
| `-32001` | 权限拒绝 | 插件权限未覆盖的宿主 API 调用。 |
| `-32002` | 不支持 | 插件没有实现已声明能力中的某个可选方法。 |
| `-32003` | 配置无效 | 用户输入的值缺失或错误。请把 `data.field` 设为该字段的键。 |

::: tip 提示
`-32003` 与 `-32000` 的区别很重要：Nginx UI 会把 `-32003` 显示在出错的字段旁边，方便用户修正；而 `-32000` 被视为服务故障。不要在 `-32000` 到 `-32099` 范围内自行定义其他代码。
:::

## JSON 结构 {#json-shapes}

每个 `params` 和 `result` 都是 plugin-spec 仓库中 [proto 文件](https://github.com/nginxui/plugin-spec/tree/main/proto/nginxui/plugin/v1)所定义消息的 [protobuf JSON 映射](https://protobuf.dev/programming-guides/json/)。本指南的各个页面都以 JSON 展示每条消息，所以只有在生成代码时才需要 proto 文件。

- 成员名使用 `snake_case`，例如 `effective_fqdn`，从不使用 `camelCase`。
- 缺失的成员与取默认值（`""`、`0`、`false`、`[]`、`{}`、`null`）的成员含义相同。发送方可以省略默认值，接收方把缺失的成员视为默认值。
- 数字是 JSON 数字。可能超过 4 GiB 的字节数是双精度浮点数，在 2^53 以内是精确的；`5242880` 和 `5242880.0` 都要接受。
- 二进制数据使用带填充的 base64。
- 没有参数的方法可以省略 `params`，没有结果的方法回复 `{}`。
- **忽略不认识的成员。** 协议正是通过新增可选成员来扩展而不破坏旧插件的。

## 连接的生命周期 {#connection-lifetime}

连接就是进程的标准输入和标准输出。进程退出、任意一方关闭自己那一端，或者消息过大时，连接结束。新进程意味着新连接和新握手。

## gRPC 传输 {#grpc-transport}

插件还可以通过 [gRPC](https://grpc.io) 提供能力调用。方法、消息、错误和结果都不变，只是传输方式不同。Nginx UI 能用时就用 gRPC，否则退回标准输入输出，因此插件在标准输入输出上也必须继续提供每个方法。

### 启用 {#opting-in}

插件在握手回复的 `transports` 中列出 `grpc`，并说明监听位置：

```json
{
  "jsonrpc": "2.0", "id": 1,
  "result": {
    "api_version": 1,
    "capabilities": ["dns01"],
    "transports": ["stdio", "grpc"],
    "rpc_socket": "/var/lib/nginx-ui/plugins/.data/com.example.mydns/rpc.sock"
  }
}
```

- **Unix 套接字。** 插件默认监听 `<NGINX_UI_PLUGIN_DATA_DIR>/rpc.sock`。套接字路径在 macOS 和 BSD 上限制为 104 字节，在 Linux 上为 108 字节；默认路径放不下时，插件在系统临时目录下创建一个私有目录（权限 `0700`）并在其中监听，然后在 `rpc_socket` 中报告路径。监听前删除残留的套接字文件，以 `0600` 权限创建套接字，并在退出时删除它。
- **命名管道。** 在 Windows 上，插件以随机名称创建一个命名管道并在 `rpc_pipe` 中报告，同时报告 `rpc_token`（至少 128 位的随机值）。管道遵循与 [HTTP 管道](./http.md#where-to-listen)相同的规则。Nginx UI 在每次调用中发送 `authorization: Bearer <rpc_token>`，插件以 `UNAUTHENTICATED` 拒绝不带这个值的调用。
- **回环 TCP。** 无法打开命名管道的 Windows 插件改为监听 `127.0.0.1`，并报告 `rpc_port` 和 `rpc_token`，令牌检查相同。

监听必须在插件回复握手之前就能接受连接。随后 Nginx UI 通过 gRPC 发送 `plugin.ping` 检查通道；插件没有列出 `grpc` 或检查失败时，该进程在整个生命周期内都使用标准输入输出。

### 各类流量的传输方式 {#what-travels-where}

| 流量 | 传输方式 |
| --- | --- |
| 握手、`plugin.initialized`、`plugin.shutdown`、`plugin.exit` | 始终使用标准输入输出 |
| `plugin.configure` 和健康检查 | 标准输入输出 |
| 宿主 API 调用及其回复 | 标准输入输出 |
| 事件和定时调用 | 标准输入输出 |
| 能力调用（`dns01.*`、`http.handle`、`notify.*`、`probe.check`、`mcp.call`、`storage.*`、`deploy.*`、`blocklist.fetch`、`discovery.resolve`） | 通道正常时使用 gRPC，否则使用标准输入输出 |
| 流（`log.push`） | 只使用 gRPC |

提供 gRPC 的插件必须在 gRPC 上提供每个能力方法和 `plugin.ping`，并且在两种传输方式上返回相同的结果或错误。如果握手和停止方法通过 gRPC 到达，插件应忽略它们。

进程运行期间通道断开时，Nginx UI 会在该进程剩余的生命周期内改用标准输入输出。以 `UNAVAILABLE` 失败且不带插件错误详情的调用会在标准输入输出上重试一次，所以插件自己不要返回 `UNAVAILABLE`。

### gRPC 消息与错误 {#grpc-messages-and-errors}

gRPC 服务路径来自 proto 文件，例如 `/nginxui.plugin.v1.DNS01/Present`，消息使用标准的 protobuf 编码，因此生成的桩代码可以直接使用。双向都接受最大 64 MiB 的消息。Nginx UI 会把自己的截止时间作为 gRPC 截止时间传递，截止时间到达后请停止处理该调用。

失败的调用带有 gRPC 状态码，并附带一个 `nginxui.plugin.v1.PluginError` 状态详情，其中包含 JSON-RPC 的 `code` 和 `data`：

| JSON-RPC 代码 | gRPC 状态码 |
| --- | --- |
| `-32601` 方法不存在、`-32002` 不支持 | `UNIMPLEMENTED` |
| `-32602` 参数无效、`-32003` 配置无效 | `INVALID_ARGUMENT` |
| `-32001` 权限拒绝 | `PERMISSION_DENIED` |
| `-32000` 内部错误 | `INTERNAL` |
| 其他代码 | `UNKNOWN` |

接收方优先使用详情中的代码，没有详情时再根据状态码反向映射。

### 流 {#streams}

有些流量是连续的数据流而不是一次调用，目前唯一的是 `log.push`，它把访问日志行交给插件（参见[访问日志推送](./capabilities/log-sink.md)）。它是客户端流式 gRPC 调用：Nginx UI 发送任意数量的消息后关闭发送端，插件回复一次。流没有 JSON-RPC 形式：它从不通过标准输入输出传输，在那里它的方法名会像任何未知方法一样得到 `-32601`。
