---
outline: [2, 3]
---

# 插件目录

Nginx UI 的插件市场列出一个或多个**插件目录**中的插件。插件目录是描述插件及其插件包位置的静态 JSON 文档。官方插件目录位于 `https://plugins.nginxui.com`，任何人都可以发布自己的插件目录，供其他人添加为来源。

## 发布到官方插件目录 {#publishing-in-the-official-catalog}

官方插件目录由 [nginxui/plugins](https://github.com/nginxui/plugins) 仓库构建。提交插件的步骤：

1. 在公开仓库中发布插件，并创建一个以签名后的插件包为附件的 GitHub Release。
2. 确保 `nginx-ui plugin lint`，以及（对有服务端进程的插件）`nginx-ui plugin conformance` 能够通过。仓库会对你的版本运行这两项检查。
3. 使用 **Submit a plugin** 表单创建 issue，它会为你起草条目；或者直接提交一个添加 `plugins/<插件 ID>.json` 的拉取请求。

仓库的贡献指南介绍了审核流程。之后的新版本会自动从你的仓库获取。

## 目录文档 {#catalog-document}

```json [index.json]
{
  "schema_version": 1,
  "name": { "en": "Example Plugins", "zh_CN": "示例插件" },
  "icon": "https://plugins.example.com/assets/icon.png",
  "updated_at": "2026-09-30T00:00:00Z",
  "plugins": [ ... ]
}
```

| 字段 | 含义 |
| --- | --- |
| `schema_version` | `1`。 |
| `name` | 各语言的目录名称，以 `en` 为后备。Nginx UI 在任何提到插件来源的地方都显示它。 |
| `icon` | 代表该目录的方形图片。 |
| `updated_at` | 此版本文档的时间。 |
| `plugins` | 条目。 |

[JSON Schema](https://github.com/nginxui/plugin-spec/blob/main/schema/catalog.schema.json) 描述了所有成员。不认识的成员会被忽略。

### 部署位置 {#where-to-serve-it}

把文档放在站点的 `/v1/index.json`，例如 `https://plugins.example.com/v1/index.json`。这样用户只需输入 `plugins.example.com` 即可添加来源：Nginx UI 会依次在 `/v1/index.json` 和 `/index.json` 查找目录，并保留有响应的地址。

### 名称与图标 {#name-and-icon}

目录自己决定自己的名称。Nginx UI 在插件市场和来源列表中显示它，会合并多余的空白，并可能把它截短到 64 个字符。没有名称时显示地址中的主机。

图标是 PNG、SVG 或 WebP 图片，方形，除矢量图外宽度至少 64 像素。它按照[截图](#screenshots)的规则加载，只能来自目录自己的主机或 GitHub；其他位置的图标会被丢弃，并显示通用图片。

## 条目 {#entries}

```json [index.json]
{
  "id": "io.github.example.mydns",
  "name": { "en": "MyDNS" },
  "description": { "en": "DNS-01 challenges through the MyDNS API." },
  "author": "example",
  "author_public_key": "RWQ...",
  "repository_url": "https://github.com/example/mydns",
  "readme_url": "https://raw.githubusercontent.com/example/mydns/main/README.md",
  "icon_url": "https://raw.githubusercontent.com/example/mydns/main/icon.png",
  "categories": ["dns01"],
  "capabilities": ["dns01"],
  "license": "MIT",
  "trust": "community",
  "stage": "production",
  "releases": [ ... ]
}
```

| 字段 | 含义 |
| --- | --- |
| `id` | 插件 ID。 |
| `name`、`description` | 各语言的文本，必须包含 `en`。 |
| `author` | 插件的发布者。 |
| `author_public_key`、`trust` | 参见[发布者](#publisher)。 |
| `homepage_url`、`repository_url`、`readme_url`、`icon_url` | 链接。README 会显示在插件详情中。 |
| `screenshots` | 参见[截图](#screenshots)。 |
| `categories`、`capabilities` | 用于筛选插件市场。 |
| `license` | 插件许可证的 SPDX 标识。 |
| `stage` | `production` 或 `beta`，显示在插件旁边的标签。 |
| `releases` | 各个版本，见下文。 |

条目的链接和图片只使用目录主机、插件包主机或 GitHub 上的 `https` 地址。

## 版本 {#releases}

```json [index.json]
{
  "version": "1.2.0",
  "released_at": "2026-09-30T00:00:00Z",
  "api_version": 1,
  "min_nginx_ui_version": "2.7.0",
  "platforms": ["linux-amd64", "linux-arm64"],
  "downloads": {
    "linux-amd64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-amd64.tar.gz", "sha256": "..." },
    "linux-arm64": { "url": "https://github.com/example/mydns/releases/download/v1.2.0/io.github.example.mydns-1.2.0-linux-arm64.tar.gz", "sha256": "..." }
  },
  "release_notes_url": "https://github.com/example/mydns/releases/tag/v1.2.0",
  "manifest": { ... }
}
```

| 字段 | 含义 |
| --- | --- |
| `version` | 该版本的插件版本号。 |
| `api_version` | 协议代次，`1`。 |
| `min_nginx_ui_version` | 与清单中的含义相同。 |
| `platforms` | 该版本可以安装的平台：`downloads` 的所有键加上通用插件包支持的平台。为空表示所有平台。 |
| `downloads` | 按平台键列出的插件包，`any` 表示可在所有平台运行的插件包。每项包含 `url` 和 `sha256`。 |
| `download_url`、`sha256` | 通用插件包，`downloads` 未列出的平台都使用它。 |
| `channel` | `stable`、`beta` 或 `dev`。参见[发布渠道](#release-channels)。 |
| `yanked` | 该版本已撤回，不再提供。 |
| `manifest` | 该版本的 `plugin.json`。 |

版本至少包含 `downloads` 和 `download_url` 中的一个。`downloads` 中某个平台的条目指向该平台的插件包，`any` 条目指向可在所有平台运行的插件包。请为每个下载提供 `sha256`。

### Nginx UI 如何选择插件包 {#how-nginx-ui-picks-a-package}

在平台 `P` 上，Nginx UI 依次选择 `downloads[P]`、`downloads["any"]`，以及在 `platforms` 为空或包含 `P` 或 `any` 时的 `download_url`。都不匹配时，该版本无法在 `P` 上安装。它在所有场合都使用同样的选择方式：选择最新版本、检查更新、解析依赖，以及为其他节点或离线安装下载插件包。

下载后，它先检查 `sha256`，再检查[签名](./signing.md)，解压后再确认清单的 ID 和版本与条目和版本一致。校验和只说明下载到的是目录所指的文件，只有签名才能说明是谁发布了它。

### 清单快照 {#the-manifest-snapshot}

`manifest` 是整个版本的清单：它的 `server.executables` 列出该版本提供的所有平台，尽管每个单一平台的插件包只声明自己的平台。Nginx UI 在下载任何内容之前，就根据它显示权限、依赖和平台。DNS-01 服务商列表这类较大的块可以省略。

## 发布者 {#publisher}

| 字段 | 含义 |
| --- | --- |
| `author_public_key` | 为插件包签名的 minisign 公钥，即 `.pub` 文件中的 base64 那一行。从该条目下载并由它签名的插件包是 `community`。 |
| `trust` | `official`、`verified` 或 `community`：目录预期的等级。仅用于列出和筛选，从不授予等级。只有保留命名空间 `com.nginxui.*` 中的 ID 才会显示为 `official`，其他 ID 显示为 `community`。 |

`author_public_key` 只对从携带它的条目下载的插件包有效。参见[签名与信任](./signing.md#trust-levels)。

## 发布渠道 {#release-channels}

每个版本都属于一个渠道，从最稳定到最不稳定依次为：`stable`、`beta` 和 `dev`。没有 `channel` 成员时，由版本号决定：

- 没有预发布部分的版本是 `stable`；
- 预发布部分的第一个标识符为 `alpha`、`dev`、`nightly`、`snapshot`、`canary` 或 `preview`（不区分大小写）的版本是 `dev`，因此 `1.0.0-nightly.20260930` 是 `dev`，而 `1.0.0-alphabet` 不是；
- 其他预发布版本，例如 `1.0.0-beta.1` 或 `2.0.0-rc.1`，是 `beta`。

设置 `channel` 可以把普通版本号放到较不稳定的渠道。条目的 `stage` 不会改变任何渠道。

::: tip 提示
请用点分隔的数字为预发布版本编号，例如 `1.1.0-beta.10`：只有单独成段时标识符才按数字比较，所以 `1.1.0-beta10` 会排在 `1.1.0-beta9` 之前。
:::

用户如何收到版本：

- 每个已安装的插件跟随一个渠道，除非用户选择其他渠道，否则为 `stable`。更新来自所跟随的渠道，或者在已安装版本的渠道更不稳定时来自该渠道，并且包含所有更稳定的版本。跟随 `beta` 的用户也会收到结束一系列测试版的正式版。
- 安装单个测试版不会改变所跟随的渠道。这样的安装会收到更新的测试版，直到安装了正式版，然后回到稳定渠道。只有测试版的插件也是如此：用户先得到测试版，然后得到第一个正式版。
- 对于尚未安装的插件，Nginx UI 选择最新的稳定版，没有时选择最新的 `beta`，再没有时选择最新的 `dev`，并标记没有稳定版的插件。
- 任何未撤回且能在本机运行的版本都可以按版本号安装。安装较旧的版本属于降级，只有在用户要求时才会发生，并会警告新版本写入的数据可能无法被旧版本读取。

## 截图 {#screenshots}

```json [index.json]
"screenshots": [
  {
    "url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials.png",
    "dark_url": "https://raw.githubusercontent.com/example/mydns/main/docs/credentials-dark.png",
    "caption": { "en": "The credential form" }
  }
]
```

最多八张插件使用时的 PNG、JPEG 或 WebP 图片，按显示顺序排列，每张可以附带各语言的说明。宽度 1280 到 1920 像素、宽高比约 16:10 的图片适合所有屏幕。Nginx UI 只从目录主机、插件包主机或 GitHub 上的 `https` 地址加载它们，丢弃其余图片，并且永远不会把图片缺失视为条目的问题。

`dark_url` 是可选的，内容是同一画面的深色主题版本。界面为深色时 Nginx UI 显示它，否则显示 `url` 的图片；没有深色图片，或深色图片无法加载时，两种主题都显示浅色图片。两张图片应拍摄同一画面，尺寸相同。

## 托管自己的插件目录 {#hosting-a-catalog-of-your-own}

插件目录是静态文件，任何 Web 服务器或静态托管服务都可以：

- 通过 HTTPS 在 `/v1/index.json` 提供它。
- 把插件包、图片和 README 放在同一主机或 GitHub 上。
- 为目录设置 `name` 和 `icon`。
- 为插件包签名：在开发者模式之外，未签名的插件包永远无法安装。目录中的 `author_public_key` 让你的插件包成为 `community`，运维人员还需要允许社区插件。

用户在 **插件 > 市场 > 来源** 中添加插件目录。只有官方插件目录可以发布合作伙伴密钥。

插件目录按列表顺序合并，插件从第一个列出它的目录安装。官方插件目录无法移除，默认排在最前面，也可以调整位置：排在它前面的镜像会代替它提供官方插件。这只改变插件包的下载位置，不改变插件包本身：`com.nginxui.*` 中的 ID 只会从官方密钥签名的插件包安装，当这类插件由其他目录提供时，插件市场会标出来源。
