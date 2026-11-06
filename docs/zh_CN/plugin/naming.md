---
outline: [2, 3]
---

# 命名规则

插件选用的一些名称会与其他所有插件共享，因此需要遵循本页的规则。这些名称一旦发布，都不应再更改。

## 插件 ID {#plugin-ids}

插件 ID 是一个反向域名：两段或多段以点分隔的小写字母、数字和连字符，最长 64 个字符，最重要的一段在前：

```text
com.example.mydns
io.github.example.mydns
```

- 使用你拥有的域名，并将其反转。
- 如果没有自己的域名，请使用 `io.github.<owner>.<name>`，其中 `<owner>` 是你的 GitHub 用户名或组织名（小写）。每个 GitHub 用户都已经拥有这个命名空间。
- `com.nginxui.*` 保留给 Nginx UI 项目的插件，官方插件目录会拒绝其他人使用它。无论插件包来自哪个插件目录或文件，Nginx UI 只安装由官方密钥签名的此类插件。开发者模式下不受此限制，以便构建官方插件。

::: warning 注意
ID 永久标识一个插件：它的设置、数据、证书和目录条目都与 ID 关联。改名就意味着发布一个新插件。
:::

## 服务商与类型代码 {#provider-and-kind-codes}

向 Nginx UI 表单添加选项的能力使用 `code` 标识每个选项：

| 能力 | 代码标识的对象 |
| --- | --- |
| `dns01` | DNS 服务商 |
| `notify` | 通知渠道 |
| `probe` | 健康检查类型 |
| `storage` | 存储后端 |
| `cert.deploy` | 部署目标类型 |
| `security.blocklist` | 封禁列表来源类型 |
| `upstream.discovery` | 发现提供方 |

代码匹配 `^[a-z0-9-]{2,32}$`，且在一个清单中只出现一次。代码在同一能力的所有已安装插件之间共享：证书、通知渠道或备份任务只保存代码，使用时 Nginx UI 再找到拥有该代码的插件。因此：

- **用服务商或技术命名，而不是用插件命名**：用 `cloudflare`，不要用 `mydns-cloudflare`。绝不要给代码加上插件 ID 前缀。
- **避免使用其他插件已经在用的代码**，尤其是官方 DNS-01 插件的服务商代码，除非你的插件就是要替代那个实现。如果另一个已启用的插件已经注册了某个代码，Nginx UI 可能会拒绝启用你的插件。
- **绝不要重命名已发布的代码。** 使用旧代码配置的所有内容都会失效。

多个已启用的插件声明同一个代码时，Nginx UI 使用插件 ID 最小的那一个。插件的代码永远不会与 Nginx UI 自己的渠道、检查或存储冲突，它们是分开管理的。

## MCP 工具名称 {#mcp-tool-names}

MCP 工具名称匹配 `^[a-z0-9][a-z0-9_-]{0,47}$`，且在插件内唯一。Nginx UI 发布工具时会在前面加上插件 ID（点替换为下划线），中间用两个下划线连接：

```text
io.github.example.cdn + purge_cache  →  io_github_example_cdn__purge_cache
```

有些模型 API 只接受 64 个字符，因此请让发布后的名称不超过 64 个字符。不要重命名已发布的工具，因为客户端和提示词会按名称引用它。

## 设置键与插槽名称 {#settings-keys-and-slot-names}

设置键只属于一个插件，所以使用 `timeout` 这样的短键没有问题。

插件为自己定义的插槽（不属于 [Nginx UI 插槽](./slots.md)的插槽）应以插件 ID 作为前缀，例如 `io.github.example.mydns:extra-panel`，以免与 Nginx UI 或其他插件将来的插槽冲突。

## 平台键 {#platform-keys}

平台键的格式是 `<goos>-<goarch>`：Go 工具链的 `GOOS` 和 `GOARCH` 值用连字符连接，与 `go tool dist list` 的输出一致，例如 `linux-amd64`、`linux-arm64`、`darwin-arm64`、`windows-amd64`。它们用于 `server.executables`、插件目录中版本的 `platforms` 和 `downloads`，以及插件包文件名。`any` 在插件目录中表示所有平台，但不能用于文件名。

## 插件包文件名 {#package-file-names}

```text
<id>-<version>.tar.gz                  通用插件包
<id>-<version>-<goos>-<goarch>.tar.gz  单一平台的插件包
```

例如 `com.nginxui.dns01-1.0.0.tar.gz` 和 `com.nginxui.dns01-1.0.0-linux-arm64.tar.gz`。只有当文件名最后两段（以连字符分隔）是已知的 GOOS 和 GOARCH 时，才会被视为单一平台的插件包，因此不要发布预发布部分以这样一对结尾的版本，例如 `1.0.0-linux-amd64`。

Nginx UI 从不依赖文件名：它需要的一切都来自包内的清单，上传的插件包可以使用任何名称。
