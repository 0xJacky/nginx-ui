# 安装脚本

此 shell 脚本仅适用于 Linux 系统。如果您使用的是其他操作系统，请参考 [快速入门](./getting-started) 指南进行手动安装或使用 Docker。

## 安装或升级

### `install.sh install`

安装或更新 Nginx UI。

### 用法

```shell
install.sh install [OPTIONS]
```

### 选项

| 选项                    |                                                                                       |
|-----------------------|---------------------------------------------------------------------------------------|
| `-l, --local <file>`  | 从本地文件安装 Nginx UI (`string`)                                                           |
| `-p, --proxy <url>`   | 通过代理服务器下载 (`string`)<br/>例如：`-p http://127.0.0.1:8118` 或 `-p socks5://127.0.0.1:1080` |
| `-r, --reverse-proxy` | 通过反向代理服务器下载 (`string`)<br/>例如：`-r https://cloud.nginxui.com/`                               |
| `-c, --channel <channel>` | 指定版本通道 (`string`)<br/>可用通道：`stable`（默认）、`prerelease`、`dev`

#### 版本通道

| 通道         | 描述                                                      |
|------------|-----------------------------------------------------------|
| `stable`   | 最新稳定版本（默认） - 推荐用于生产环境                                |
| `prerelease` | 最新预发布版本 - 包含正在测试的新功能，将在稳定版本发布前进行验证                |
| `dev`      | 来自 dev 分支的最新开发构建 - 包含最新功能但可能不稳定                   |

### 快速使用

::: code-group

```shell [稳定版（默认）]
# 安装最新稳定版本
bash -c "$(curl -L https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install -r https://cloud.nginxui.com/
```

```shell [预发布版]
# 安装最新预发布版本
bash -c "$(curl -L https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install --channel prerelease -r https://cloud.nginxui.com/
```

```shell [开发版]
# 安装最新开发构建
bash -c "$(curl -L https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install --channel dev -r https://cloud.nginxui.com/
```

:::

一键安装脚本默认设置的监听端口为 `9000`，HTTP Challenge 端口默认为 `9180`。如果有端口冲突，请手动修改 `/usr/local/etc/nginx-ui/app.ini`，
并使用 `systemctl restart nginx-ui` 重启 Nginx UI 服务。更多有关信息，请查看 [配置参考](./config-server)。

## 卸载

### `install.sh remove`

卸载 Nginx UI。

### 用法

```shell
install.sh remove [OPTIONS]
```

### 选项

| 选项        |                                       |
|-----------|---------------------------------------|
| `--purge` | 删除所有 Nginx UI 文件，包括日志、配置等 (`boolean`) |

### 快速使用

::: code-group

```shell [移除]
# 删除 Nginx UI，但不包括配置和数据库文件
bash -c "$(curl -L https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove
```

```shell [清除]
# 删除所有 Nginx UI 文件，包括配置和数据库文件
bash -c "$(curl -L https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove --purge
```

:::

## 帮助

### `install.sh help`

显示可用选项。

### 用法

```shell
install.sh help
```

### 快速使用

```shell
bash -c "$(curl -L -s https://cloud.nginxui.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ help
```

## 控制服务

通过此脚本，Nginx UI 将作为服务安装。安装脚本会检测您系统的服务管理器并设置相应的服务控制机制。

### Systemd

如果您的系统使用 systemd，请使用以下 `systemctl` 命令对其进行控制：

::: code-group

```shell [启动]
systemctl start nginx-ui
```

```shell [停止]
systemctl stop nginx-ui
```

```shell [重启]
systemctl restart nginx-ui
```

```shell [显示状态]
systemctl status nginx-ui
```

```shell [开机启动]
systemctl enable nginx-ui
```

:::

### OpenRC

如果您的系统使用 OpenRC，请使用以下 `rc-service` 命令对其进行控制：

::: code-group

```shell [启动]
rc-service nginx-ui start
```

```shell [停止]
rc-service nginx-ui stop
```

```shell [重启]
rc-service nginx-ui restart
```

```shell [显示状态]
rc-service nginx-ui status
```

```shell [开机启动]
rc-update add nginx-ui default
```

:::

### Init.d

如果您的系统使用传统的 init.d 脚本，请使用以下命令对其进行控制：

::: code-group

```shell [启动]
/etc/init.d/nginx-ui start
```

```shell [停止]
/etc/init.d/nginx-ui stop
```

```shell [重启]
/etc/init.d/nginx-ui restart
```

```shell [显示状态]
/etc/init.d/nginx-ui status
```

:::
