---
outline: [2, 3]
---

# 存储

`storage` 插件把文件保存到 Nginx UI 自身无法访问的地方：WebDAV 共享、SFTP 服务器、云盘。它的后端会出现在内置存储旁边，Nginx UI 把它们用作自动备份的目的地。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `storage.put` | 是 | 以某个键保存文件。 |
| `storage.get` | 是 | 把某个键下的对象取回到文件中。 |
| `storage.list` | 是 | 列出键以某个前缀开头的对象。 |
| `storage.delete` | 是 | 删除某个键下的对象。 |
| `storage.validate` | 否 | 在不保存任何内容的情况下检查配置。 |

文件内容从不在消息中传输。Nginx UI 与插件以文件的形式交换内容，因此几 GB 的备份也不会在任何一方的内存中多占一份。插件应请求 `network`。

## 声明后端 {#declaring-backends}

```json [plugin.json]
"capabilities": ["storage"],
"permissions": ["network"],
"storage": {
  "backends": [
    {
      "code": "webdav",
      "name": "WebDAV",
      "configuration": {
        "fields": [
          { "key": "url", "display_name": "Server URL", "help_text": "Folder the objects are kept in", "required": true },
          { "key": "username", "display_name": "Username", "required": true },
          { "key": "password", "display_name": "Password", "required": true, "secret": true }
        ]
      }
    }
  ]
}
```

`code`、`name` 和 `configuration.fields` 的用法与[通知渠道](./notify.md#configuration-form)相同。

## 键 {#keys}

键命名一个对象。它是由 `/` 分隔的相对路径，最长 1024 字节，不能为空，不能以 `/` 开头或结尾，不能包含空段、`.` 或 `..` 段、`\` 或控制字符。键区分大小写，使用 UTF-8，可以包含任何文字。请按需把键映射成服务自己的命名方式（例如百分号编码），并在 `storage.list` 中原样返回。对其他形式的键回复 `-32602`。

## 交换文件 {#exchanging-files}

文件通过插件数据目录中的 `exchange` 目录传递。每次需要传递文件的调用，Nginx UI 都会新建一个随机命名的子目录，并传入其中的绝对路径：

- 对于 `storage.put`，调用前文件已位于 `source_path`。只能读取它：不要修改、移动或删除。
- 对于 `storage.get`，`target_path` 尚不存在。请把完整的对象创建为一个普通文件，失败时删除不完整的文件。

::: warning 注意
不要访问交换目录中的任何其他路径，回复之后也不要保留或使用这些路径：调用返回后 Nginx UI 会删除该子目录。无法打开或创建文件的插件回复 `-32000`。
:::

## 保存 {#storing}

```json
{
  "jsonrpc": "2.0", "id": 42, "method": "storage.put",
  "params": {
    "backend": "webdav",
    "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" },
    "key": "nginx-ui/daily_1790000000.zip",
    "source_path": "/var/lib/nginx-ui/plugins/.data/io.github.example.webdav/exchange/5f0c2a9d/daily_1790000000.zip"
  }
}
```

以 `key` 保存完整的文件，替换已有的对象，并且只在它持久保存后才回复：`{ "size": <已保存的字节数> }`。保存失败时不应留下不完整的对象。原因在于配置（密码错误、目录不存在）时回复带 `data.field` 的 `-32003`，其他情况（服务故障、磁盘已满、网络故障）回复 `-32000`。

## 取回 {#fetching}

`storage.get` 包含 `backend`、`config`、`key` 和 `target_path`。把完整的对象写入 `target_path`，并回复 `{ "size": <已写入的字节数> }`。键下没有对象时回复 `-32000`，并在消息中说明。

## 列出 {#listing}

::: code-group

```json [请求]
{ "jsonrpc": "2.0", "id": 44, "method": "storage.list", "params": { "backend": "webdav", "config": {}, "prefix": "nginx-ui/daily_" } }
```

```json [响应]
{ "jsonrpc": "2.0", "id": 44, "result": { "objects": [
  { "key": "nginx-ui/daily_1790000000.zip", "size": 5242880, "modified_at": "2026-09-21T03:00:05Z" }
] } }
```
:::

`prefix` 是普通的字符串前缀而不是目录：`nginx-ui/daily_` 匹配 `nginx-ui/daily_1.zip`，空前缀列出所有对象。列出所有匹配项（自行处理服务的分页），顺序不限。`modified_at` 是 RFC 3339 时间，服务不提供时为空。能区分时，请排除不是由你保存的对象。没有匹配时返回空列表。

## 删除 {#deleting}

`storage.delete` 包含 `backend`、`config` 和 `key`。删除对象并回复 `{}`。删除不存在对象的键也要成功，这样 Nginx UI 可以重试清理。

## 校验 {#validating}

`storage.validate` 包含 `backend` 和 `config`。在不保存、修改或删除任何内容的情况下检查配置，最好也不要访问服务。对第一个问题回复带 `data.field` 的 `-32003`，否则回复 `{}`。要确认服务是否接受这份配置，Nginx UI 会调用 `storage.list`。

绝不要把 `config` 的值或文件内容写入日志。

## 备份如何使用它 {#how-backups-use-it}

- 备份任务的存储路径就是键前缀。每次运行保存 `<prefix>/<name>_<unix time>.zip`，加密备份还会保存 `<prefix>/<name>_<unix time>.zip.key`。
- 只保留最近若干份备份的任务，在每次运行后用自己的前缀调用 `storage.list`，并对较旧的备份调用 `storage.delete`。
- 恢复时对归档文件及其密钥文件调用 `storage.get`。
- 备份表单的测试按钮先调用 `storage.validate`，再调用 `storage.list`。

`storage.put` 和 `storage.get` 最多等待 10 分钟，其他方法最多等待 30 秒，Nginx UI 不会自行重试任何操作。
