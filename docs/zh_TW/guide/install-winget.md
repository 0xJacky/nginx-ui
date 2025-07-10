# 使用 Winget 安裝

此安裝方法適用於已安裝 Windows 套件管理員 (winget) 的 Windows 使用者。

## 前提條件

- **Windows**: Windows 10 版本 1709 (組建 16299) 或更高版本
- 已安裝 [Windows 套件管理員 (winget)](https://learn.microsoft.com/zh-tw/windows/package-manager/winget/)

如果您尚未安裝 winget，可以從 [Microsoft Store](https://www.microsoft.com/store/productId/9NBLGGH4NNS1) 安裝或從 [GitHub 發布頁面](https://github.com/microsoft/winget-cli/releases) 下載。

## 安裝

### 安裝 Nginx UI

```powershell
winget install 0xJacky.nginx-ui
```

此命令將：
- 下載並安裝最新穩定版本的 Nginx UI 到 `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- 設定必要的相依性
- 將 nginx-ui 新增到系統 PATH

**注意**：安裝過程不會建立任何設定檔案。您需要手動建立設定檔案，或讓 Nginx UI 在首次執行時建立。

### 驗證安裝

安裝完成後，您可以驗證 Nginx UI 是否正確安裝：

```powershell
nginx-ui --version
```

### 安裝目錄

WinGet 將 Nginx UI 安裝到使用者本機目錄：
- **安裝路徑**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\`
- **可執行檔路徑**: `%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe`

您可以使用以下命令存取此目錄：
```powershell
cd "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\"
```

## 服務管理

在 Windows 上，Nginx UI 可以作為 Windows 服務執行，也可以從命令列手動啟動。

### 安裝為 Windows 服務

由於 Nginx UI 沒有內建的 Windows 服務管理功能，您需要使用 `sc.exe` 手動註冊：

```powershell
# 建立服務（以系統管理員身分執行）
# 注意：WinGet 安裝到使用者本機目錄
sc create nginx-ui binPath= "%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe serve" start= auto

# 啟動服務
sc start nginx-ui
```

### 手動服務管理

您可以使用 Windows 服務管理員或 PowerShell 管理服務：

```powershell
# 啟動服務
Start-Service nginx-ui

# 停止服務
Stop-Service nginx-ui

# 重啟服務
Restart-Service nginx-ui

# 檢查服務狀態
Get-Service nginx-ui
```

### 設定服務自動啟動

在上面的建立命令中已經透過 `start= auto` 設定了服務自動啟動。如需後續修改：

```powershell
Set-Service -Name nginx-ui -StartupType Automatic
```

## 手動執行

如果您更喜歡手動執行 Nginx UI 而不是作為服務：

```powershell
# 在前台執行
nginx-ui

# 使用自訂設定執行
nginx-ui serve -config C:\path\to\your\app.ini

# 直接從安裝目錄執行
"%LOCALAPPDATA%\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve

# 在背景執行（使用 Start-Job）
Start-Job -ScriptBlock { nginx-ui serve }
```

## 設定

設定檔案需要手動建立或將在首次執行時建立，應位於：

- **預設路徑**: `%LOCALAPPDATA%\nginx-ui\app.ini`
- **替代路徑**: `C:\ProgramData\nginx-ui\app.ini`

資料通常儲存在：
- `%LOCALAPPDATA%\nginx-ui\`
- `C:\ProgramData\nginx-ui\`

### 建立設定

您可以選擇：
1. **讓 Nginx UI 自動建立** - 首次執行 nginx-ui，它會在當前工作目錄建立預設設定
2. **手動建立** - 自己建立目錄和設定檔案

手動建立設定目錄和檔案：
```powershell
# 建立設定目錄
New-Item -ItemType Directory -Force -Path "$env:LOCALAPPDATA\nginx-ui"

# 建立基本設定檔案
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

預設設定包含：
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

### 更新所有軟體套件

```powershell
winget upgrade --all
```

## 解除安裝

### 停止並解除安裝服務

```powershell
# 首先停止服務
sc stop nginx-ui

# 刪除服務
sc delete nginx-ui

# 解除安裝軟體套件
winget uninstall nginx-ui
```

### 刪除設定和資料

::: warning 警告

這將永久刪除您的所有設定、站點、憑證和資料。請確保在繼續之前備份任何重要資料。

:::

```powershell
# 刪除設定和資料目錄
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\nginx-ui"
Remove-Item -Recurse -Force "$env:PROGRAMDATA\nginx-ui"
```

## 故障排除

### 連接埠衝突

如果遇到連接埠衝突（預設連接埠為 9000），您需要修改設定檔案：

1. **編輯設定檔案：**
   ```powershell
   notepad "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

2. **在 `[server]` 部分更改連接埠：**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **重啟服務：**
   ```powershell
   Restart-Service nginx-ui
   ```

### Windows 防火牆

如果您在從其他裝置存取 Nginx UI 時遇到問題，可能需要設定 Windows 防火牆：

```powershell
# 允許 Nginx UI 通過 Windows 防火牆（TCP 和 UDP）
New-NetFirewallRule -DisplayName "Nginx UI TCP" -Direction Inbound -Protocol TCP -LocalPort 9000 -Action Allow
New-NetFirewallRule -DisplayName "Nginx UI UDP" -Direction Inbound -Protocol UDP -LocalPort 9000 -Action Allow
```

### 檢視服務日誌

要排查服務問題，您可以檢視日誌：

#### Windows 事件檢視器

1. 開啟事件檢視器 (`eventvwr.msc`)
2. 導航到 Windows 日誌 > 應用程式
3. 查詢來源為 "nginx-ui" 的事件

#### 服務日誌

如果 Nginx UI 設定為將日誌寫入檔案：

```powershell
# 檢視日誌檔案（如果已設定）
Get-Content "$env:LOCALAPPDATA\nginx-ui\logs\nginx-ui.log" -Tail 50
```

### 權限問題

如果遇到權限問題：

1. **以系統管理員身分執行：** 某些操作可能需要系統管理員權限
2. **檢查資料夾權限：** 確保 Nginx UI 對其設定和資料目錄具有讀/寫存取權限
3. **防毒軟體：** 某些防毒程式可能會干擾 Nginx UI 的執行

### 服務無法啟動

如果服務啟動失敗：

1. **檢查服務狀態：**
   ```powershell
   Get-Service nginx-ui
   ```

2. **驗證設定檔案是否存在（如需要則建立）：**
   ```powershell
   Test-Path "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # 如果返回 False，請先建立設定目錄和檔案
   ```

3. **嘗試手動執行以檢視錯誤訊息：**
   ```powershell
   nginx-ui serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   # 或直接從安裝目錄執行：
   & "$env:LOCALAPPDATA\Microsoft\WinGet\Packages\0xJacky.nginx-ui__DefaultSource\nginx-ui.exe" serve -config "$env:LOCALAPPDATA\nginx-ui\app.ini"
   ```

4. **檢查連接埠衝突：**
   ```powershell
   # 檢查連接埠 9000 是否已被佔用
   netstat -an | findstr :9000
   ```

## 取得協助

如果遇到任何問題：

1. 查看 [官方文件](https://nginxui.com)
2. 在 [GitHub](https://github.com/0xJacky/nginx-ui/issues) 上搜尋現有問題
3. 如果問題尚未回報，請建立新的問題

## 下一步

安裝完成後，您可以：

1. 在 `http://localhost:9000` 存取 Web 介面
2. 完成初始設定精靈
3. 開始設定您的 Nginx 站點
4. 探索 [設定指南](./config-server) 進行進階設定
