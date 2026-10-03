---
outline: [2, 3]
---

# 访问日志推送

`log.sink` 插件在 nginx 写入访问日志时实时接收 Nginx UI 的访问日志行（已解析为字段），并把它们发送到任何地方：日志存储、SIEM、指标管道。Nginx UI 读取日志，把日志行分批，并以流的方式把每一批发给插件。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `log.push` | 是 | 接收一批条目并回复一次。仅限 gRPC。 |

访问日志产生的行数远多于每行一个请求所能承载的量，因此 `log.push` 是 gRPC 流，`log.sink` 插件必须提供 [gRPC 传输](../protocol.md#grpc-transport)。访问日志包含客户端地址和每个请求的 URL，因此插件必须请求 `log.read` 权限。

## 声明能力 {#declaring-the-capability}

```json [plugin.json]
"capabilities": ["log.sink"],
"permissions": ["log.read", "network"],
"log_sink": {
  "batch_size": 512,
  "flush_interval_ms": 1000,
  "formats": ["combined"]
}
```

`log_sink` 块是可选的：

| 字段 | 默认值 | 含义 |
| --- | --- | --- |
| `batch_size` | 256 | 一个流中最多的条目数，0 到 4096。 |
| `flush_interval_ms` | 500 | 流在第一个条目之后保持打开的最长时间。`0` 或至少 50。 |
| `formats` | 所有行 | 插件需要的日志行格式。 |

流中的条目达到 `batch_size`，或者自第一个条目起已过去 `flush_interval_ms`，流就会立即关闭。

| 格式 | 日志行 |
| --- | --- |
| `combined` | nginx `combined` 格式的行，后面可以跟 `$request_time` 和 `$upstream_response_time`。解析出的字段都会设置。 |
| `raw` | 其他所有行：自定义 `log_format`、JSON 日志、被截断的行。只设置 `raw` 和 `timestamp`（读取该行的时间）。 |

列出了格式的插件永远不会收到其他格式的行，包括将来新增的格式。

## 接收流 {#receiving-a-stream}

插件在握手中列出 `grpc`，并提供 `/nginxui.plugin.v1.LogSink/Push`。Nginx UI 打开一个流，每行发送一条消息，然后关闭发送端；插件读到末尾后回复一次。一条消息的 JSON 形式如下：

```json
{
  "log_path": "/var/log/nginx/access.log",
  "entry": {
    "timestamp": "2026-09-23T08:15:02Z",
    "remote_addr": "203.0.113.7",
    "request_method": "GET",
    "request_uri": "/index.html?lang=en",
    "protocol": "HTTP/1.1",
    "status": 200,
    "body_bytes_sent": 612,
    "referer": "https://example.com/",
    "user_agent": "Mozilla/5.0 (X11; Linux x86_64)",
    "request_time": 0.004,
    "upstream_response_time": 0.003,
    "raw": "203.0.113.7 - - [23/Sep/2026:08:15:02 +0000] \"GET /index.html?lang=en HTTP/1.1\" 200 612 ...",
    "format": "combined"
  }
}
```

回复统计插件保留和丢弃的条目数：

```json
{ "accepted": 3, "rejected": 0 }
```

### 条目字段 {#entry-fields}

| 字段 | nginx 变量 | 含义 |
| --- | --- | --- |
| `timestamp` | `$time_local` | 请求时间，RFC 3339，UTC。 |
| `remote_addr` | `$remote_addr` | 客户端地址。 |
| `request_method`、`request_uri`、`protocol` | 来自 `$request` | 方法、带查询字符串的目标、协议。 |
| `status` | `$status` | 响应状态。 |
| `body_bytes_sent` | `$body_bytes_sent` | 响应体大小，单位字节。 |
| `referer`、`user_agent` | `$http_referer`、`$http_user_agent` | 请求头。 |
| `upstream_addr` | `$upstream_addr` | 处理请求的上游服务器。 |
| `request_time`、`upstream_response_time` | 同名变量 | 时间，单位秒。 |
| `host` | `$host` | 请求所属的主机。 |
| `raw` | | nginx 写入的原始行，总会设置。 |
| `format` | | `combined` 或 `raw`。 |

Nginx UI 无法提取的字段不会出现。`combined` 格式既不包含 `$upstream_addr` 也不包含 `$host`，需要它们的插件请自行解析 `raw`。

### 及时回复 {#answering-promptly}

在收到回复之前，Nginx UI 不会打开下一个流，等待期间还会丢弃日志行，所以请尽快回复：先缓存条目，在目的地确认之前就回复。错误状态表示插件丢失了整批条目，这批条目不会重新发送。

::: warning 注意
每个字段都是不可信的输入，因为目标、来源页和用户代理都由客户端决定；在许多司法管辖区，这些条目还属于个人数据。绝不要把它们写入自己的日志。
:::

## Nginx UI 如何推送 {#how-nginx-ui-streams}

- 它推送插件启用期间写入的日志行，只来自 Nginx UI 日志查看器被允许读取的日志。它从不回放较早的行，也不会在日志轮转时重复发送同一行。
- 慢的插件永远不会拖慢 nginx 或其他插件：每个插件有一个 8192 条的队列，放不下的行会被丢弃并计数。
- 失败的流中的条目计为丢弃，下一个流等待 1 秒，每次失败后加倍，最长 30 秒。每个流最长 30 秒。
- 插件详情显示它接受和拒绝的条目数，以及 Nginx UI 丢弃的条目数。不提供 gRPC 的 `log.sink` 插件收不到任何内容，并会报告原因。
