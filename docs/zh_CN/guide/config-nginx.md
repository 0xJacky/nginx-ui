# Nginx

在本节中，我们将会介绍 Nginx UI 中关于 Nginx 控制命令、日志路径等参数的配置选项。

::: tip 提示
自 v2.0.0-beta.3 版本起，我们将 `nginx_log` 配置项改名为 `nginx`。
:::


## 日志
Nginx 日志对于监控、排查问题和维护您的 Web 服务器至关重要。它们提供了有关服务器性能、用户行为和潜在问题的宝贵见解。

### AccessLogPath

- 类型：`string`

此选项用于为 Nginx UI 设置 Nginx 访问日志的路径，以便我们在线查看日志内容。

::: tip 提示
在 v2 版本中，我们会读取 `nginx -V` 命令的输出，以获取 Nginx 访问日志的默认路径。

如果您需要设置不同的路径，您可以使用此选项。
:::

### ErrorLogPath

- 类型：`string`

此选项用于为 Nginx UI 设置 Nginx 错误日志的路径，以便我们在线查看日志内容。

::: tip 提示
在 v2 版本中，我们会读取 `nginx -V` 命令的输出，以获取 Nginx 错误日志的默认路径。

如果您需要设置不同的路径，您可以使用此选项。
:::

### LogDirWhiteList

- 类型：`[]string`
- 版本：`>= v2.0.0-beta.36`
- 示例：`/var/log/nginx,/var/log/sites`

此选项用于为 Nginx UI 设置日志查看器的目录白名单。

::: warning 警告
出于安全原因，您必须指定存储日志的目录。

只有这些目录中的日志可以在线查看。
:::

## 服务监控与控制

在本节中，我们将会介绍 Nginx UI 中关于 Nginx 服务的监控和控制命令的配置选项。

### ConfigDir
- 类型：`string`

此选项用于设置 Nginx 配置文件夹的路径。

在 v2 版本中，我们会读取 `nginx -V` 命令的输出，以获取 Nginx 配置文件的默认路径。

如果您需要覆盖默认路径，您可以使用此选项。

### PIDPath
- 类型：`string`

此选项用于设置 Nginx PID 文件的路径。Nginx UI 将通过判断该文件是否存在来判断 Nginx 服务的运行状态。

在 v2 版本中，我们会读取 `nginx -V` 命令的输出，以获取 Nginx PID 文件的默认路径。

如果您需要覆盖默认路径，您可以使用此选项。

### SbinPath
- 类型：`string`
- 版本：`>= v2.1.10`

此选项用于设置 Nginx 可执行文件的路径。

默认情况下，Nginx UI 会尝试在 `$PATH` 中查找 Nginx 可执行文件。

如果您需要覆盖默认路径，您可以使用此选项。

### TestConfigCmd
- 类型：`string`
- 默认值：`nginx -t`

此选项用于设置 Nginx 测试配置的命令。

### ReloadCmd
- 类型：`string`
- 默认值：`nginx -s reload`

此选项用于设置 Nginx 重新加载配置的命令。

### RestartCmd
- 类型：`string`

::: tip 提示
我们建议使用 systemd 管理 Nginx 的用户，将这个值设置为 `systemctl restart nginx`。
否则，当您在 Nginx UI 中重启 Nginx 后，将无法在 systemctl 中获取 Nginx 的准确状态。
:::

若此选项为空，则 Nginx UI 将使用以下命令关闭 Nginx 服务：

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

若无法从 `nginx -V` 中获得 `--sbin-path` 路径，则 Nginx UI 将使用以下命令启动 Nginx 服务：

```bash
nginx
```

若可以获取到 `--sbin-path` 路径，则 Nginx UI 将使用以下命令启动 Nginx 服务：

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```

::: tip 通过 SSH 管理宿主机模式
在通过 SSH 管理宿主机模式下，非空的 `TestConfigCmd`、`ReloadCmd` 或 `RestartCmd` 会以 SSH 用户身份通过 `/bin/sh -c` 在宿主机上执行。留空时，Nginx UI 会使用宿主机的 nginx 可执行文件进行测试，并通过 systemd 或 launchd 重载或重启。参见 [在 Docker 中管理宿主机 Nginx](manage-host-nginx-from-docker.md)。
:::

### StubStatusPort
- 类型：`uint`
- 默认值：`51820`
- 版本：`>= v2.0.0-rc.6`

此选项用于设置 Nginx stub status 模块的端口。stub status 模块提供了 Nginx 的基本状态信息，Nginx UI 使用这些信息来监控服务器的性能。

::: tip 提示
请确保您设置的端口未被其他服务占用。
:::

## 维护页面

### MaintenanceDir
- 类型：`string`
- 默认值：`/etc/nginx/maintenance`
- 环境变量：`NGINX_UI_NGINX_MAINTENANCE_DIR`

此选项用于设置 Nginx UI 读取自定义维护页面模板的目录。若留空，则使用 `/etc/nginx/maintenance`。

### MaintenanceTemplate
- 类型：`string`
- 环境变量：`NGINX_UI_NGINX_MAINTENANCE_TEMPLATE`
- 示例：`maintenance.html`

此选项用于为 Nginx UI 维护页面选择自定义 HTML 模板。您可以通过环境变量设置，也可以在 Settings > Nginx 中设置。

此配置只使用文件名，配置值中的路径部分会被忽略。Nginx UI 按以下顺序从维护目录加载模板：

1. `<MaintenanceDir>/<站点名称>.<文件名>`，即该站点专属的模板；
2. `<MaintenanceDir>/<文件名>`，即所有站点共用的通用模板。

例如当 `MaintenanceTemplate=maintenance.html` 时，站点 `example.com` 会优先尝试 `/etc/nginx/maintenance/example.com.maintenance.html`，匹配不到时降级为 `/etc/nginx/maintenance/maintenance.html`。

如果此选项为空、文件不可读或文件内容为空，Nginx UI 将回退到内置维护页面模板。

对于 Docker 部署，请将宿主机目录挂载到维护目录，并将模板文件放在该目录中：

```yaml
services:
  nginx-ui:
    image: uozi/nginx-ui:latest
    volumes:
      - ./maintenance:/etc/nginx/maintenance
    environment:
      - NGINX_UI_NGINX_MAINTENANCE_TEMPLATE=maintenance.html
```

## 容器控制

在本节中，我们将会介绍 Nginx UI 中关于控制运行在另一个 Docker 容器中的 Nginx 服务的配置选项。

### ContainerName
- 类型：`string`
- 版本：`>= v2.0.0-rc.6`

此选项用于指定运行 Nginx 的 Docker 容器名称。

如果此选项为空，Nginx UI 将控制本机或当前容器内的 Nginx 服务。

如果此选项不为空，Nginx UI 将控制运行在指定容器中的 Nginx 服务。

::: tip 提示
如果使用 Nginx UI 官方容器，想要控制另外一个容器里的 Nginx，务必将宿主机内的 docker.sock 映射到 Nginx UI 官方容器中。

例如：`-v /var/run/docker.sock:/var/run/docker.sock`

Nginx UI 通过自身的文件系统读写 Nginx 配置和日志文件。请将相同的配置和日志目录以相同路径挂载到两个容器中。配置目录在 Nginx UI 容器中必须可写，在 Nginx 容器中可以保持只读。映射 `docker.sock` 和设置 `ContainerName` 只会将状态检查和控制命令转发到另一个容器，不会自动共享文件或转发业务流量。
:::

## 通过 SSH 控制宿主机 Nginx

对于 Nginx UI 运行在 Docker 容器中、而 Nginx 以原生方式安装在宿主机上的部署场景，Nginx UI 提供了第三种控制模式，通过 SSH 执行命令，并使用 SFTP 或绑定挂载进行文件 I/O。该模式支持 Linux systemd 服务和 macOS Homebrew launchd 服务。

### 限制

::: warning 限制
- **仅限同一宿主机**：Nginx UI 容器与目标 nginx 进程必须在同一台物理机或虚拟机上。如需多主机管理，请参阅 [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)。
- Linux 上的 nginx 必须由 systemd 管理，并允许 SSH 用户通过免密码 `sudo -n` 调用一组受限命令。
- macOS 上的 nginx 必须作为 Homebrew 用户服务运行；SSH 用户必须是拥有 `homebrew.mxcl.nginx` 的登录用户，并且不会使用 sudo。
:::

### 快速开始

1. 在 Web 界面中，前往**偏好设置 → Nginx**，选择**通过 SSH 控制宿主机**模式，并打开配置向导。
2. 按照五步配置向导操作（**SSH 目标**、**信任与测试**、**检测平台**、**访问与安装**、**验证**）：选择或生成密钥对、信任主机密钥并测试连接、检测服务管理器和 nginx 路径、选择文件访问模式并应用生成的容器与宿主机片段，然后执行验证。
3. 所有检查通过后，保存配置。

也可以使用命令行：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp
nginx-ui host-setup test
```

### 配置字段

| 字段 | 描述 |
|---|---|
| `host_mode` | 设置为 `ssh` 以启用此模式 |
| `host_access_mode` | `sftp` 或 `mounted`。SSH 模式下必填：容器通过 SFTP 还是通过 bind mount 访问宿主机 nginx 文件 |
| `host_key_source` | `generated`（默认）、`existing` 或 `provided`：SSH 私钥的来源 |
| `host_address` | 远程 `host:port` |
| `host_user` | 宿主机上的 SSH 用户 |
| `host_auth_method` | SSH 认证方式。当前宿主机 SSH 配置请使用密钥认证 |
| `host_private_key_path` | 容器内的私钥路径 |
| `host_known_hosts_path` | 容器内的 known_hosts 允许列表路径 |
| `host_sudo_prefix` | 特权命令前缀。默认值为 `sudo -n` |
| `host_service_manager` | `systemd`（默认）或 `launchd` |
| `host_systemd_unit_name` | 默认为 `nginx.service` |
| `host_systemctl_path` | 默认为 `/bin/systemctl` |
| `host_launchd_service` | 默认为 `homebrew.mxcl.nginx` |
| `host_launchctl_path` | 默认为 `/bin/launchctl` |
| `host_config_dir` | 宿主机侧 nginx 配置目录 |
| `host_log_dir` | 宿主机侧 nginx 日志目录 |
| `sbin_path` | SSH 模式下可选：宿主机上的 nginx 可执行文件。留空时，Nginx UI 会解析服务管理器的默认值（systemd 为 `/usr/sbin/nginx`，launchd 为 `/opt/homebrew/opt/nginx/bin/nginx`），并在保存控制设置时写入。生成的 sudoers 允许列表会精确匹配解析后的路径 |

另请参阅：[在 Docker 中管理宿主机 Nginx](manage-host-nginx-from-docker.md) 和 [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)。
