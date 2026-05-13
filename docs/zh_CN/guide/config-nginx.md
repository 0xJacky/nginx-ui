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

### StubStatusPort
- 类型：`uint`
- 默认值：`51820`
- 版本：`>= v2.0.0-rc.6`

此选项用于设置 Nginx stub status 模块的端口。stub status 模块提供了 Nginx 的基本状态信息，Nginx UI 使用这些信息来监控服务器的性能。

::: tip 提示
请确保您设置的端口未被其他服务占用。
:::

## 维护页面

### MaintenanceTemplate
- 类型：`string`
- 环境变量：`NGINX_UI_NGINX_MAINTENANCE_TEMPLATE`
- 示例：`maintenance.html`

此选项用于为 Nginx UI 维护页面选择自定义 HTML 模板。您可以通过环境变量设置，也可以在 Settings > Nginx 中设置。

此配置只使用文件名。Nginx UI 会从 `/etc/nginx/maintenance/<filename>` 加载自定义模板，配置值中的路径部分会被忽略。

如果此选项为空、文件不可读或文件内容为空，Nginx UI 将回退到内置维护页面模板。

对于 Docker 部署，请将宿主机目录挂载到 `/etc/nginx/maintenance`，并将模板文件放在该目录中：

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
:::
