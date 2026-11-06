# 升级到 v3

按原来的安装方式正常升级即可，其余迁移由 Nginx UI v3 在首次启动时完成。本页说明磁盘上的变化，以及无外网节点需要注意的事项。

## DNS-01 证书

DNS-01 证书使用的 DNS 服务商已从 Nginx UI 中移出，改由官方插件 `com.nginxui.dns01` 提供。因此 Nginx UI 的二进制文件更小，空闲时占用的内存也更少。

无需任何操作。节点上有 DNS-01 证书时，Nginx UI 会在启动时自动安装并启用该插件，之后每天再检查一次。安装包按以下顺序查找：

1. 离线安装包目录 `<配置目录>/plugins/packages`；
2. 发行版附带的安装包。官方 Docker 镜像内置了该插件，无论是否有 DNS-01 证书，容器首次启动时都会安装它；
3. 插件市场。

插件安装完成后，之前等待该插件的证书会立即续期。如果无法安装，Nginx UI 会发出通知，在插件安装之前 DNS-01 证书不会续期。

DNS 凭证会保留。[RecursiveNameservers](./config-cert.md#recursivenameservers) 的值会在插件首次安装时复制到插件设置中。

### 无外网的节点

如果节点运行的是官方 Docker 镜像，内置的插件就足够了。否则，请在一台能访问外网并安装了 Nginx UI v3 的机器上下载安装包：

```shell
nginx-ui plugin fetch com.nginxui.dns01 --platform linux-amd64
```

平台请填写目标节点的 `<os>-<arch>`，例如 `linux-arm64`。把下载的 `.tar.gz` 文件复制到节点的 `<配置目录>/plugins/packages` 中，然后启动或重启 Nginx UI。也可以在 **系统 > 插件** 中通过 **从文件安装** 上传安装包。

## 日志索引

日志分析仍然内置在 Nginx UI 中。索引从 Bleve 改为列式存储，占用的磁盘空间小得多。如果开启了索引，v3 首次启动时会从索引目录（默认为 `app.ini` 同一目录下的 `log-index`）中删除旧的索引目录，并把所有日志重新索引到其中的 `logstore` 子目录。重建期间，搜索、仪表盘和地图的数据会逐步补全。详见 [Nginx Log](./config-nginx-log.md)。

## 插件目录

已安装的插件保存在插件目录中，默认为 `app.ini` 同一目录下的 `plugins`：

| 安装方式 | 插件目录 |
| --- | --- |
| Docker | `/etc/nginx-ui/plugins`，位于 `/etc/nginx-ui` 卷内 |
| Linux 安装脚本 | `/usr/local/etc/nginx-ui/plugins` |
| 其他 | `app.ini` 所在目录下的 `plugins` |

请把该目录与配置目录的其他内容一起保留。插件相关选项见 [Plugin](./config-plugin.md)。
