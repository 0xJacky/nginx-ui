---
outline: [2, 3]
---

# 官方插件目录

官方插件目录位于 `https://plugins.nginxui.com`，是每个 Nginx UI 默认提供的插件市场。插件在[开发者中心](https://portal.nginxui.com)提交、审核和管理。本页介绍这一流程；插件目录的格式以及如何搭建自己的插件目录，参见[插件目录](./catalog.md)。

## 提交前的准备 {#before-you-submit}

你的插件需要：

- 一个公开的 GitHub 仓库，至少有一个 GitHub Release，其 tag 是 `v1.0.0` 这样的语义化版本。Release 的附件就是插件包：通用的 `<id>-<version>.tar.gz`、每个平台一个的 `<id>-<version>-<goos>-<goarch>.tar.gz`，或两者都有，每个插件包旁边附带一个 `.sha256` 文件。参见[打包](./packaging.md)。
- 能通过 `nginx-ui plugin lint` 的 `plugin.json`；有服务端进程的插件还要通过 `nginx-ui plugin conformance`。插件目录会对你的版本运行这两项检查。
- 符合[命名规则](./naming.md)的插件 ID。没有自己的域名时，使用 `io.github.<owner>.<name>`。`com.nginxui.` 命名空间保留给 Nginx UI 项目自己的插件。
- 一把主密钥以及由它认证的签名密钥，使用 `nginx-ui plugin key init --id <插件 ID>` 创建一次即可；插件包用 `nginx-ui plugin pack --key signing.key` 签名。参见[签名密钥](./signing.md#signing-keys)。

## 提交 {#submitting}

1. 使用 GitHub 登录[开发者中心](https://portal.nginxui.com)，选择**提交**。
2. 选择仓库。仓库需要同意被收录：你需要对它有管理员权限，或者在它上面安装了 [NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog) GitHub App。
3. 粘贴你的主公钥，即 `primary.pub` 中的 base64 那一行。它会成为目录条目的 `author_public_key`，决定哪些插件包是 `community`，参见[信任等级](./signing.md#trust-levels)。
4. 检查会在你的最新版本上运行，并同时给出结果和上架后的展示效果：
   - 条目符合目录 schema，插件 ID 唯一；对于 `io.github.<owner>.<name>` 形式的 ID，仓库属于 `<owner>`；
   - 每个插件包都与 GitHub 记录的 sha256 一致，`plugin.sums` 列出了所有文件，并带有由你的主密钥为该插件认证过的签名密钥所做的签名；
   - 在 linux-amd64 节点会安装的那个插件包上，`lint` 和 `conformance` 能够通过。
5. 提交。变更页面会跟踪提交所处的阶段（已提交、检查、审核、已合并、已上线），并显示当前在等待谁。你可以撤回提交；检查失败后，发布修复版本即可重试。

不能通过验证的版本不会进入插件目录；最新版本通过验证之前，提交无法上架。

## 审核 {#review}

维护者对每个提交审核一次：

1. 仓库同意被收录。
2. 名称没有冒充其他插件或 Nginx UI 项目。每种语言的名称都会审核，任何名称都不能声称是官方的。
3. 权限和能力与插件的功能相符，并且 `permission_reasons` 说明了原因。
4. README 和截图与插件本身相关，并遵守行为准则。
5. 许可证和分类合适。

审核通过后，插件目录会重新构建，插件会在一小时内出现在每个节点的插件市场中；用户手动刷新插件市场时会立即出现。

## 上架之后 {#after-listing}

之后的版本不会逐个审核。插件目录每小时读取一次你的 GitHub Releases，新版本的插件包一旦通过验证就会上架。预发布版本会进入其版本号所指的[发布渠道](./catalog.md#release-channels)，草稿 Release 在发布之前会被忽略。

如果希望版本在几分钟内上架，可以只在插件仓库上安装 [NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog) GitHub App。之后发布 Release 会立即触发插件目录构建。这个 App 只接收 Release 事件，不持有私钥，也无法读取你的仓库。

已上架的版本是固定的：在 GitHub 上替换它的某个插件包，不会改变节点下载到的内容，只会让插件目录构建失败。请发布新版本。

### 版本会改变什么 {#what-your-releases-change}

上架内容跟随最新的、未被撤回的稳定版本；还没有稳定版本时，跟随最新版本：

| 展示内容 | 来自该版本的 |
| --- | --- |
| 名称 | `plugin.json` 的 `name` 和 `i18n.<语言>.name`，经维护者批准后生效。 |
| 描述 | `plugin.json` 的 `description` 和 `i18n.<语言>.description`。 |
| 主页 | `plugin.json` 的 `homepage_url`。 |
| README | 该版本 tag 上的 `README.md`。 |
| 图标 | 插件包中由 `icon_path` 指定的文件：不超过 256 KB 的 PNG、WebP 或 SVG。 |
| 截图 | `plugin.json` 的 `screenshots`，参见[截图](./manifest.md#screenshots)。 |

当某个版本在任何语言中使用了上架内容中还没有的名称时，上架内容会保留原有名称，直到维护者批准新名称。包含"官方"一类字样或不可见字符的名称会被忽略。

描述不需要审核，但如果一句话里同时提到 Nginx UI 并声称是官方的，例如"Nginx UI 官方 DNS 插件"，这条描述会被忽略，上架内容保留原有描述。"Cloudflare 官方 API"这样的写法没有问题。

## 管理上架内容 {#managing-a-listing}

开发者中心里插件的页面包括：

- **商店**、**截图**和**翻译**：插件市场展示的内容，预览使用与 Nginx UI 相同的组件绘制。修改会作为变更提交，可以在变更页面跟踪。
- **版本**：撤回某个版本，或恢复已撤回的版本。已撤回的版本不会再提供给尚未安装它的节点，正在运行它的节点会收到更新或移除的提示。不要删除已撤回版本的 GitHub Release：已经安装它的节点仍需要在插件目录中看到它。
- **签名者**：吊销可能已经泄露的签名密钥。由它签名的所有版本都会被撤回，节点会把它签名的插件包视为未签名，所以请用新密钥签名并发布新版本。只是想更换没有泄露的签名密钥时，运行 `nginx-ui plugin key rotate` 即可，插件目录无需任何改动。
- **访问权限**：还有谁可以管理这个上架条目。

撤回版本、恢复版本和吊销签名者无需审核即可生效。更换主密钥（用于主密钥丢失或泄露的情况）需要维护者审核，并且会撤回所有已上架的版本，因为它们的签名者证书将不再能通过验证。

## 合作伙伴插件 {#partner-plugins}

`verified` 信任等级用于 Nginx UI 维护者担保的合作伙伴组织。它表明插件包由谁发布，并不表示有人审查过其源代码。

1. 在开发者中心你的组织页面提交申请。双方达成一致后，生成一对 minisign 密钥（`minisign -G -p partner.pub -s partner.key`），只发送 `partner.pub`。
2. 维护者把这把公钥加入签名的合作伙伴密钥环，并发回一份合作伙伴证书：`plugin.partner` 和 `plugin.partner.minisig`，其中写明你的组织，并且可以带有过期日期。
3. 在写入 `plugin.sums` 之前，把这两个文件原样放到每个插件包的根目录，然后用你的合作伙伴密钥为 `plugin.sums` 签名。

当证书能用节点固定的官方密钥验证，或者节点的密钥环副本列出了你的公钥时，节点会把这样的插件包作为 `verified` 安装。请在过期日期之前申请新证书。如果密钥可能已经泄露，请通过[安全公告](https://github.com/0xJacky/nginx-ui/security/advisories/new)私下报告：维护者会在密钥环中吊销它，节点在下一次刷新插件目录时就不再信任它。

## 已上架插件的安全问题 {#security-problems-in-a-listed-plugin}

已上架插件中的漏洞、凭据泄露或许可证违规，请通过[安全公告](https://github.com/0xJacky/nginx-ui/security/advisories/new)私下报告，不要公开提 issue。对于安全问题，维护者可以不等作者，直接撤回版本或下架插件，事后再通知作者。
