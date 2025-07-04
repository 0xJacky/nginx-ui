# 使用 Homebrew 安装

此安装方法适用于已经安装了 Homebrew 的 macOS 和 Linux 用户。

## 前提条件

- **macOS**: macOS 11 Big Sur 或更高版本 (amd64 / arm64)
- **Linux**: 大多数现代 Linux 发行版（Ubuntu、Debian、CentOS 等）
- 系统已安装 [Homebrew](https://brew.sh/)

如果您尚未安装 Homebrew，可以使用以下命令安装：

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

## 安装

### 安装 Nginx UI

```bash
brew install 0xjacky/tools/nginx-ui
```

此命令将：
- 将 `0xjacky/tools` tap 添加到您的 Homebrew
- 下载并安装最新稳定版本的 Nginx UI
- 设置必要的依赖项
- 创建默认配置文件和目录

### 验证安装

安装完成后，您可以验证 Nginx UI 是否正确安装：

```bash
nginx-ui --version
```

## 服务管理

Nginx UI 可以使用 Homebrew 的服务管理功能作为系统服务进行管理。

### 启动服务

```bash
# 启动服务并设置开机自启
brew services start nginx-ui

# 或者仅为当前会话启动服务
brew services run nginx-ui
```

### 停止服务

```bash
brew services stop nginx-ui
```

### 重启服务

```bash
brew services restart nginx-ui
```

### 检查服务状态

```bash
brew services list | grep nginx-ui
```

## 手动运行

如果您更喜欢手动运行 Nginx UI 而不是作为服务：

```bash
# 在前台运行
nginx-ui

# 使用自定义配置运行
nginx-ui serve -config /path/to/your/app.ini

# 在后台运行
nohup nginx-ui serve &
```

## 配置

配置文件在安装过程中自动创建，位于：

- **macOS (Apple Silicon)**: `/opt/homebrew/etc/nginx-ui/app.ini`
- **macOS (Intel)**: `/usr/local/etc/nginx-ui/app.ini`
- **Linux**: `/home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini`

数据存储在：
- **macOS (Apple Silicon)**: `/opt/homebrew/var/nginx-ui/`
- **macOS (Intel)**: `/usr/local/var/nginx-ui/`
- **Linux**: `/home/linuxbrew/.linuxbrew/var/nginx-ui/`

默认配置包含：
```ini
[app]
PageSize = 10

[server]
Host = 0.0.0.0
Port = 9000
RunMode = release

[cert]
HTTPChallengePort = 9180

[terminal]
StartCmd = login
```

## 更新

### 更新 Nginx UI

```bash
brew upgrade nginx-ui
```

### 更新 Homebrew 和所有软件包

```bash
brew update && brew upgrade
```

## 卸载

### 停止并卸载

```bash
# 首先停止服务
brew services stop nginx-ui

# 卸载软件包
brew uninstall nginx-ui
```

### 移除 Tap（可选）

如果您不再需要该 tap：

```bash
brew untap 0xjacky/tools
```

### 删除配置和数据

::: warning 警告

这将永久删除您的所有配置、站点、证书和数据。请确保在继续之前备份任何重要数据。

:::

```bash
# macOS (Apple Silicon)
sudo rm -rf /opt/homebrew/etc/nginx-ui/
sudo rm -rf /opt/homebrew/var/nginx-ui/

# macOS (Intel)
sudo rm -rf /usr/local/etc/nginx-ui/
sudo rm -rf /usr/local/var/nginx-ui/

# Linux
sudo rm -rf /home/linuxbrew/.linuxbrew/etc/nginx-ui/
sudo rm -rf /home/linuxbrew/.linuxbrew/var/nginx-ui/
```

## 故障排除

### 端口冲突

如果遇到端口冲突（默认端口为 9000），您需要修改配置文件：

1. **编辑配置文件：**
   ```bash
   # macOS (Apple Silicon)
   sudo nano /opt/homebrew/etc/nginx-ui/app.ini

   # macOS (Intel)
   sudo nano /usr/local/etc/nginx-ui/app.ini

   # Linux
   sudo nano /home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini
   ```

2. **在 `[server]` 部分更改端口：**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **重启服务：**
   ```bash
   brew services restart nginx-ui
   ```

### 查看服务日志

要排查服务问题，您可以使用以下命令查看日志：

#### Homebrew 服务日志

Nginx UI 的 Homebrew 配方包含了正确的日志配置：

```bash
# 查看服务状态和日志文件路径
brew services info nginx-ui

# 查看标准输出日志
tail -f $(brew --prefix)/var/log/nginx-ui.log

# 查看错误日志
tail -f $(brew --prefix)/var/log/nginx-ui.err.log

# 同时查看两个日志文件
tail -f $(brew --prefix)/var/log/nginx-ui.log $(brew --prefix)/var/log/nginx-ui.err.log
```

#### systemd 日志 (Linux)

对于使用 systemd 的 Linux 系统：

```bash
# 查看服务日志
journalctl -u homebrew.mxcl.nginx-ui -f

# 查看最近的日志
journalctl -u homebrew.mxcl.nginx-ui --since "1 hour ago"
```

#### 手动调试

如果需要调试服务问题，可以手动运行以查看输出：

```bash
# 在前台运行以查看所有输出
nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini

# 检查服务是否正在运行
ps aux | grep nginx-ui
```

### 权限问题

如果在管理 Nginx 配置时遇到权限问题：

1. 确保您的用户具有读写 Nginx 配置文件的必要权限
2. 对于某些操作，您可能需要以提升的权限运行 Nginx UI
3. 检查文件权限：
   ```bash
   # 检查配置文件权限
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # 检查数据目录权限
   ls -la $(brew --prefix)/var/nginx-ui/
   ```

### 服务无法启动

如果服务启动失败：

1. **检查服务状态：**
   ```bash
   brew services list | grep nginx-ui
   ```

2. **验证配置文件是否存在且有效：**
   ```bash
   # 检查配置文件是否存在
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # 测试配置
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini --help
   ```

3. **尝试手动运行以查看错误消息：**
   ```bash
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini
   ```

4. **检查端口冲突：**
   ```bash
   # 检查端口 9000 是否已被占用
   lsof -i :9000

   # 检查 HTTP 质询端口是否被占用
   lsof -i :9180
   ```

## 获取帮助

如果遇到任何问题：

1. 查看 [官方文档](https://nginxui.com)
2. 在 [GitHub](https://github.com/0xJacky/nginx-ui/issues) 上搜索现有问题
3. 如果您的问题尚未报告，请创建新问题

## 下一步

安装完成后，您可以：

1. 访问 `http://localhost:9000` 的 Web 界面
2. 完成初始设置向导
3. 开始配置您的 Nginx 站点
4. 探索 [配置指南](./config-server) 进行高级设置
