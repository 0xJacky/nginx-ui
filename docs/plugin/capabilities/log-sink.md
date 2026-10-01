---
outline: [2, 3]
---

# Access Log Streaming

A `log.sink` plugin receives the nginx access log lines of Nginx UI while
nginx writes them, parsed into fields, and ships them wherever it likes: a
log store, a SIEM, a metrics pipeline. Nginx UI reads the logs, batches the
lines and streams each batch to the plugin.

| Method | Required | Purpose |
| --- | --- | --- |
| `log.push` | yes | Receive one batch of entries and answer once. gRPC only. |

An access log produces far more lines than one request per line could carry,
so `log.push` is a gRPC stream and a `log.sink` plugin must serve the
[gRPC transport](../protocol.md#grpc-transport). Access logs hold client
addresses and every requested URL, so the plugin must request the `log.read`
permission.

## Declaring the Capability

```json [plugin.json]
"capabilities": ["log.sink"],
"permissions": ["log.read", "network"],
"log_sink": {
  "batch_size": 512,
  "flush_interval_ms": 1000,
  "formats": ["combined"]
}
```

The `log_sink` block is optional:

| Field | Default | Meaning |
| --- | --- | --- |
| `batch_size` | 256 | Most entries in one stream, 0 to 4096. |
| `flush_interval_ms` | 500 | Longest time a stream stays open after its first entry. `0` or at least 50. |
| `formats` | every line | The formats of the lines the plugin wants. |

A stream closes as soon as it carries `batch_size` entries or
`flush_interval_ms` passed since its first entry.

| Format | Lines |
| --- | --- |
| `combined` | Lines in the nginx `combined` format, optionally followed by `$request_time` and `$upstream_response_time`. The parsed fields are set. |
| `raw` | Every other line: a custom `log_format`, a JSON log, a truncated line. Only `raw` and `timestamp` (the time the line was read) are set. |

A plugin that lists formats never receives lines of another format, including
formats added later.

## Receiving a Stream

The plugin lists `grpc` in its handshake and serves
`/nginxui.plugin.v1.LogSink/Push`. Nginx UI opens a stream, sends one message
per line and closes its side; the plugin reads to the end and answers once.
One message, in its JSON form:

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

The answer counts what the plugin kept and discarded:

```json
{ "accepted": 3, "rejected": 0 }
```

### Entry Fields

| Field | nginx variable | Meaning |
| --- | --- | --- |
| `timestamp` | `$time_local` | Time of the request, RFC 3339, in UTC. |
| `remote_addr` | `$remote_addr` | Client address. |
| `request_method`, `request_uri`, `protocol` | from `$request` | Method, target with query string, protocol. |
| `status` | `$status` | Response status. |
| `body_bytes_sent` | `$body_bytes_sent` | Response body size in bytes. |
| `referer`, `user_agent` | `$http_referer`, `$http_user_agent` | Request headers. |
| `upstream_addr` | `$upstream_addr` | Upstream servers that handled the request. |
| `request_time`, `upstream_response_time` | same names | Times in seconds. |
| `host` | `$host` | Host the request was served for. |
| `raw` | | The line as nginx wrote it. Always set. |
| `format` | | `combined` or `raw`. |

A field Nginx UI could not extract is absent. The `combined` format carries
neither `$upstream_addr` nor `$host`; a plugin that needs them parses `raw`.

### Answering Promptly

Nginx UI does not open the next stream before the answer and drops lines
while it waits, so answer quickly: buffer the entries and answer before your
destination acknowledges them. An error status means the plugin lost the
whole batch; it is not sent again.

::: warning
Every field is untrusted input, since clients choose the target, referer and
user agent, and the entries are personal data in many jurisdictions. Never
write them to your own logs.
:::

## How Nginx UI Streams

- It streams the lines written while the plugin is enabled, from the logs
  the Nginx UI log viewer is allowed to read. It never replays older lines
  and never sends a line twice across log rotation.
- A slow plugin never holds back nginx or other plugins: each plugin has a
  queue of 8192 entries, and lines that do not fit are dropped and counted.
- A failed stream counts its entries as dropped, and the next one waits 1
  second, doubling after each failure up to 30 seconds. A stream is bounded
  to 30 seconds.
- The plugin's details show how many entries it accepted and rejected and how
  many Nginx UI dropped. A `log.sink` plugin without gRPC receives nothing and
  reports why.
