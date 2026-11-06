# Plugin

- 版本：`>= v3.0.0`

插件为 Nginx UI 增加能力，例如 DNS-01 证书所需的 DNS 服务商。插件在 **系统 > 插件** 中管理，本节的选项显示在 **偏好设置 > 插件** 中。如何编写插件，请参阅 [插件开发指南](/zh_CN/plugin/overview)。

## 安装插件

插件可以通过以下方式安装：

- 从插件页面的 **市场** 标签页安装；
- 通过 **从文件安装** 上传安装包（`.tar.gz`），或执行 `nginx-ui plugin install <安装包>`；
- 离线安装：把安装包复制到插件目录下的 `packages` 目录（默认为 `<配置目录>/plugins/packages`）。Nginx UI 下次启动时，如果安装包的版本比已安装的版本新，就会安装它，然后把安装包移到 `packages/installed`。

要为无法访问外网的节点准备安装包，可以在能访问插件市场的机器上执行 `nginx-ui plugin fetch <插件 ID> --platform <os>-<arch>`，例如 `--platform linux-amd64`。使用 `--platform all` 会下载所有平台的安装包。

每个安装包都带有签名，签名者决定了它的信任级别：

| 级别 | 签名者 | 安装条件 |
| --- | --- | --- |
| `official` | Nginx UI 项目 | 直接安装 |
| `verified` | 项目认可的合作伙伴 | 直接安装 |
| `community` | 插件作者 | 开启 [AllowCommunityPlugins](#allowcommunityplugins) 时，确认后安装 |
| `unsigned` | 无签名或未知密钥 | 仅在 [DeveloperMode](#developermode) 下可以安装 |

插件运行前会列出它申请的权限，在你批准之前一直保持停用。详见 [签名与信任](/zh_CN/plugin/signing)。

## DNS-01 插件

从 v3.0.0 起，DNS-01 证书使用的 DNS 服务商不再内置在 Nginx UI 中，而是由官方插件 `com.nginxui.dns01` 提供，安装后约占用 60 MB。

当有证书使用 DNS-01 验证而该插件尚未安装时，Nginx UI 会在启动时以及每天自动安装它。安装包依次从离线 `packages` 目录、[BundledDir](#bundleddir) 和插件市场获取，并且只接受 `official` 级别的安装包。官方 Docker 镜像内置了该插件，容器首次启动时会自动安装并启用。

卸载该插件后，除非仍有证书需要 DNS-01，否则不会被重新安装。

## Enabled

- 类型：`boolean`
- 默认值：`true`

整个插件系统的开关。关闭后会停止所有插件，并隐藏插件相关页面。

## Dir

- 类型：`string`
- 默认值：`app.ini` 所在目录下的 `plugins`

已安装插件以及离线安装用的 `packages` 目录所在的目录。在官方 Docker 镜像中为 `/etc/nginx-ui/plugins`，位于 `/etc/nginx-ui` 卷内。此选项只能在 `app.ini` 中或通过环境变量设置。

## BundledDir

- 类型：`string`
- 默认值：空

发行版随程序附带的只读插件安装包目录。每个节点会安装并启用其中的插件一次，并在附带的安装包更新时升级。被卸载的附带插件不会再次出现。此目录只会安装 `official` 级别的安装包。官方 Docker 镜像将其设置为 `/usr/local/share/nginx-ui/plugins`。此选项只能在 `app.ini` 中或通过环境变量设置。

## DefaultSyncPolicy

- 类型：`string`
- 默认值：`manual`
- 可选值：`manual`、`auto`

新安装插件的集群同步策略。`auto` 会让子节点保持安装该插件，并保持版本和启用状态一致。

## MarketplaceEnabled

- 类型：`boolean`
- 默认值：`true`

开启插件市场。关闭后只能通过文件和离线目录安装插件。

## MarketplaceSources

- 类型：`[]string`
- 默认值：空

额外插件目录的 URL，按合并顺序排列。官方目录 `https://plugins.nginxui.com/v1/index.json` 始终会被读取，并排在其他目录之前。目录来源在插件页面的市场标签页中管理。

## TrustedPublicKeys

- 类型：`[]string`
- 默认值：空

可信发布者的 minisign 公钥，每一项后面可以加一个空格和发布者名称。使用这些密钥签名的安装包以 `community` 级别安装，上传或复制到离线目录的安装包也是如此。在偏好设置中显示为 **可信发布者**。

## AllowCommunityPlugins

- 类型：`boolean`
- 默认值：`true`

允许安装 `community` 级别的插件，每次安装前都需要确认。

## AllowUploads

- 类型：`boolean`
- 默认值：`true`

允许安装通过浏览器或命令行上传的安装包。

## AutoUpdate

- 类型：`boolean`
- 默认值：`false`

在申请的权限不变时，自动更新 `official` 和 `verified` 级别的插件。

## AllowInsecureDownloadURL

- 类型：`boolean`
- 默认值：`false`

接受明文 `http` 的目录地址和下载地址。仅用于可信网络中的私有目录。

## DeveloperMode

- 类型：`boolean`
- 默认值：`false`

允许从任何来源安装未签名的插件。仅在开发插件时开启，切勿在生产节点上使用。

## MemoryLimitMB

- 类型：`int`
- 默认值：`0`（不限制）

每个插件进程的内存上限，单位 MiB。插件自身申请的上限更小时，以插件的为准。仅在使用 cgroup v2 的 Linux 上生效。

## CPUPercent

- 类型：`int`
- 默认值：`0`（不限制）

每个插件进程的 CPU 上限，以单核的百分比表示：`100` 即一个核心。插件自身申请的上限更小时，以插件的为准。仅在使用 cgroup v2 的 Linux 上生效。

## CgroupRoot

- 类型：`string`
- 默认值：`/sys/fs/cgroup`

执行资源限制所用的 cgroup v2 挂载点。每个插件对应 `<CgroupRoot>/nginx-ui/plugins/<id>`。此选项只能在 `app.ini` 中或通过环境变量设置。
