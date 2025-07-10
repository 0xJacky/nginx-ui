# 使用 Winget 安装

此安装方法适用于已安装 Windows 包管理器 (winget) 的 Windows 用户。

## 前提条件

- **Windows**: Windows 10 版本 1709 (内部版本 16299) 或更高版本
- 已安装 [Windows 包管理器 (winget)](https://learn.microsoft.com/zh-cn/windows/package-manager/winget/)

如果您尚未安装 winget，可以从 [Microsoft Store](https://www.microsoft.com/store/productId/9NBLGGH4NNS1) 安装或从 [GitHub 发布页面](https://github.com/microsoft/winget-cli/releases) 下载。

## 安装

### 安装 Nginx UI

```powershell
winget install 0xJacky.nginx-ui
```

此命令将：
- 下载并安装最新稳定版本的 Nginx UI 到 `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- 设置必要的依赖项
- 将 nginx-ui 添加到系统 PATH

**注意**：安装过程不会创建任何配置文件。您需要手动创建配置文件，或让 Nginx UI 在首次运行时创建。

### 验证安装

安装完成后，您可以验证 Nginx UI 是否正确安装：

```powershell
nginx-ui --version
```

### 安装目录

WinGet 将 Nginx UI 安装到用户本地目录：
- **安装路径**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- **可执行文件路径**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe`

您可以使用以下命令访问此目录：
```powershell
cd "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\"
```

## 服务管理

在 Windows 上，Nginx UI 可以作为 Windows 服务运行，也可以从命令行手动启动。

### 安装为 Windows 服务

由于 Nginx UI 没有内置的 Windows 服务管理功能，您需要使用 `sc.exe` 手动注册：

```powershell
# 创建服务（以管理员身份运行）
# 注意：WinGet 安装到用户本地目录
sc create nginx-ui binPath= "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe serve" start= auto

# 启动服务
sc start nginx-ui
```

### 手动服务管理

您可以使用 Windows 服务管理器或 PowerShell 管理服务：

```powershell
# 启动服务
Start-Service nginx-ui

# 停止服务
Stop-Service nginx-ui

# 重启服务
Restart-Service nginx-ui

# 检查服务状态
Get-Service nginx-ui
```

### 设置服务自动启动

在上面的创建命令中已经通过 `start= auto` 配置了服务自动启动。如需后续修改：

```powershell
Set-Service -Name nginx-ui -StartupType Automatic
```

## 手动运行

如果您更喜欢手动运行 Nginx UI 而不是作为服务：

```powershell
# 在前台运行
nginx-ui

# 使用自定义配置运行
nginx-ui serve -config C:\path\to\your\app.ini

# 直接从安装目录运行
"%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve

# 在后台运行（使用 Start-Job）
Start-Job -ScriptBlock { nginx-ui serve }
```

## 配置

配置文件需要手动创建或将在首次运行时创建，应位于：

- **默认路径**: `%LOCALAPPDATA%\nginx-ui\app.ini`
- **备选路径**: `C:\ProgramData\nginx-ui\app.ini`

数据通常存储在：
- `%LOCALAPPDATA%\nginx-ui\`
- `C:\ProgramData\nginx-ui\`

### 创建配置

您可以选择：
1. **让 Nginx UI 自动创建** - 首次运行 nginx-ui，它会在当前工作目录创建默认配置
2. **手动创建** - 自己创建目录和配置文件

手动创建配置目录和文件：
```powershell
# 创建配置目录
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\nginx-ui"

# 创建基本配置文件
@"
[app]
PageSize = 10

[server]
Host = 0.0.0.0
Port = 9000
RunMode = release

[cert]
HTTPChallengePort = 9180

[terminal]
StartCmd = cmd
"@ | Out-File -FilePath "$env:LOCALAPPDATA\nginx-ui\app.ini" -Encoding utf8
```

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
StartCmd = cmd
```

## 更新

### 更新 Nginx UI

```powershell
winget upgrade nginx-ui
```

### 更新所有软件包

```powershell
winget upgrade --all
```

## 卸载

### 停止并卸载服务

```powershell
# 首先停止服务
sc stop nginx-ui

# 删除服务
sc delete nginx-ui

# 卸载软件包
winget uninstall nginx-ui
```

### 删除配置和数据

::: warning 警告

这将永久删除您的所有配置、站点、证书和数据。请确保在继续之前备份任何重要数据。

:::

```powershell
# 删除配置和数据目录
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\nginx-ui"
Remove-Item -Recurse -Force "$env:PROGRAMDATA\nginx-ui"
```

## 故障排除

### 端口冲突

如果遇到端口冲突（默认端口为 9000），您需要修改配置文件：

1. **编辑配置文件：**
   ```powershell
   notepad "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

2. **在 `[server]` 部分更改端口：**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **重启服务：**
   ```powershell
   Restart-Service nginx-ui
   ```

### Windows 防火墙

如果您在从其他设备访问 Nginx UI 时遇到问题，可能需要配置 Windows 防火墙：

```powershell
# 允许 Nginx UI 通过 Windows 防火墙（TCP 和 UDP）
New-NetFirewallRule -DisplayName "Nginx UI TCP" -Direction Inbound -Protocol TCP -LocalPort 9000 -Action Allow
New-NetFirewallRule -DisplayName "Nginx UI UDP" -Direction Inbound -Protocol UDP -LocalPort 9000 -Action Allow
```

### 查看服务日志

要排查服务问题，您可以查看日志：

#### Windows 事件查看器

1. 打开事件查看器 (`eventvwr.msc`)
2. 导航到 Windows 日志 > 应用程序
3. 查找来源为 "nginx-ui" 的事件

#### 服务日志

如果 Nginx UI 配置为将日志写入文件：

```powershell
# 查看日志文件（如果已配置）
Get-Content "$env:LOCALAPPDATA\nginx-ui\logs\nginx-ui.log" -Tail 50
```

### 权限问题

如果遇到权限问题：

1. **以管理员身份运行：** 某些操作可能需要管理员权限
2. **检查文件夹权限：** 确保 Nginx UI 对其配置和数据目录具有读/写访问权限
3. **杀毒软件：** 某些杀毒程序可能会干扰 Nginx UI 的运行

### 服务无法启动

如果服务启动失败：

1. **检查服务状态：**
   ```powershell
   Get-Service nginx-ui
   ```

2. **验证配置文件是否存在（如需要则创建）：**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # 如果返回 False，请先创建配置目录和文件
   ```

3. **尝试手动运行以查看错误消息：**
   ```powershell
   nginx-ui serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # 或直接从安装目录运行：
   & "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

4. **检查端口冲突：**
   ```powershell
   # 检查端口 9000 是否已被占用
   netstat -an | findstr :9000
   ```

## 获取帮助

如果遇到任何问题：

1. 查看 [官方文档](https://nginxui.com)
2. 在 [GitHub](https://github.com/0xJacky/nginx-ui/issues) 上搜索现有问题
3. 如果问题尚未报告，请创建新的问题

## 下一步

安装完成后，您可以：

1. 在 `http://localhost:9000` 访问 Web 界面
2. 完成初始设置向导
3. 开始配置您的 Nginx 站点
4. 探索 [配置指南](./config-server) 进行高级设置
