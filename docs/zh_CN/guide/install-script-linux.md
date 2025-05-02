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

### 使用反向代理加速

如果您在中国大陆，可能会遇到 GitHub 的网络问题。您可以通过以下命令设置代理服务器下载 Nginx UI，以加快下载速度。

```bash
export GH_PROXY=https://ghfast.top/
```

当以上地址不可用时，请检视 [GitHub Proxy](https://ghproxy.link/) 获得最新地址，或根据实际情况选择其他代理。

### 快速使用

```shell
bash -c "$(curl -L ${GH_PROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install
```

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
bash -c "$(curl -L ${GH_PROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove
```

```shell [清除]
# 删除所有 Nginx UI 文件，包括配置和数据库文件
bash -c "$(curl -L ${GH_PROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove --purge
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
bash -c "$(curl -L -s ${GH_PROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ help
```

## 控制服务

通过此脚本，Nginx UI 将作为 `nginx-ui` 服务安装在 systemd 中。请使用以下 `systemctl` 命令对其进行控制。

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

:::
