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


