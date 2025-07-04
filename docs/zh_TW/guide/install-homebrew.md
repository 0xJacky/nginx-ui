# 使用 Homebrew 安裝

此安裝方法適用於已經安裝了 Homebrew 的 macOS 和 Linux 使用者。

## 前提條件

- **macOS**: macOS 11 Big Sur 或更高版本 (amd64 / arm64)
- **Linux**: 大多數現代 Linux 發行版（Ubuntu、Debian、CentOS 等）
- 系統已安裝 [Homebrew](https://brew.sh/)

如果您尚未安裝 Homebrew，可以使用以下命令安裝：

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

## 安裝

### 安裝 Nginx UI

```bash
brew install 0xjacky/tools/nginx-ui
```

此命令將：
- 將 `0xjacky/tools` tap 新增到您的 Homebrew
- 下載並安裝最新穩定版本的 Nginx UI
- 設定必要的相依性
- 建立預設設定檔案和目錄

### 驗證安裝

安裝完成後，您可以驗證 Nginx UI 是否正確安裝：

```bash
nginx-ui --version
```

## 服務管理

Nginx UI 可以使用 Homebrew 的服務管理功能作為系統服務進行管理。

### 啟動服務

```bash
# 啟動服務並設定開機自啟
brew services start nginx-ui

# 或者僅為目前工作階段啟動服務
brew services run nginx-ui
```

### 停止服務

```bash
brew services stop nginx-ui
```

### 重啟服務

```bash
brew services restart nginx-ui
```

### 檢查服務狀態

```bash
brew services list | grep nginx-ui
```

## 手動執行

如果您更喜歡手動執行 Nginx UI 而不是作為服務：

```bash
# 在前台執行
nginx-ui

# 使用自訂設定執行
nginx-ui serve -config /path/to/your/app.ini

# 在背景執行
nohup nginx-ui serve &
```

## 設定

設定檔案在安裝過程中自動建立，位於：

- **macOS (Apple Silicon)**: `/opt/homebrew/etc/nginx-ui/app.ini`
- **macOS (Intel)**: `/usr/local/etc/nginx-ui/app.ini`
- **Linux**: `/home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini`

資料儲存在：
- **macOS (Apple Silicon)**: `/opt/homebrew/var/nginx-ui/`
- **macOS (Intel)**: `/usr/local/var/nginx-ui/`
- **Linux**: `/home/linuxbrew/.linuxbrew/var/nginx-ui/`

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
StartCmd = login
```

## 更新

### 更新 Nginx UI

```bash
brew upgrade nginx-ui
```

### 更新 Homebrew 和所有軟體包

```bash
brew update && brew upgrade
```

## 解除安裝

### 停止並解除安裝

```bash
# 首先停止服務
brew services stop nginx-ui

# 解除安裝軟體包
brew uninstall nginx-ui
```

### 移除 Tap（可選）

如果您不再需要該 tap：

```bash
brew untap 0xjacky/tools
```

### 刪除設定和資料

::: warning 警告

這將永久刪除您的所有設定、站點、憑證和資料。請確保在繼續之前備份任何重要資料。

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

### 連接埠衝突

如果遇到連接埠衝突（預設連接埠為 9000），您需要修改設定檔案：

1. **編輯設定檔案：**
   ```bash
   # macOS (Apple Silicon)
   sudo nano /opt/homebrew/etc/nginx-ui/app.ini

   # macOS (Intel)
   sudo nano /usr/local/etc/nginx-ui/app.ini

   # Linux
   sudo nano /home/linuxbrew/.linuxbrew/etc/nginx-ui/app.ini
   ```

2. **在 `[server]` 部分更改連接埠：**
   ```ini
   [server]
   Host = 0.0.0.0
   Port = 9001
   RunMode = release
   ```

3. **重啟服務：**
   ```bash
   brew services restart nginx-ui
   ```

### 查看服務日誌

要排查服務問題，您可以使用以下命令查看日誌：

#### Homebrew 服務日誌

Nginx UI 的 Homebrew 配方包含了正確的日誌配置：

```bash
# 查看服務狀態和日誌文件路徑
brew services info nginx-ui

# 查看標準輸出日誌
tail -f $(brew --prefix)/var/log/nginx-ui.log

# 查看錯誤日誌
tail -f $(brew --prefix)/var/log/nginx-ui.err.log

# 同時查看兩個日誌文件
tail -f $(brew --prefix)/var/log/nginx-ui.log $(brew --prefix)/var/log/nginx-ui.err.log
```

#### systemd 日誌 (Linux)

對於使用 systemd 的 Linux 系統：

```bash
# 查看服務日誌
journalctl -u homebrew.mxcl.nginx-ui -f

# 查看最近的日誌
journalctl -u homebrew.mxcl.nginx-ui --since "1 hour ago"
```

#### 手動調試

如果需要調試服務問題，可以手動運行以查看輸出：

```bash
# 在前台運行以查看所有輸出
nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini

# 檢查服務是否正在執行
ps aux | grep nginx-ui
```

### 權限問題

如果在管理 Nginx 設定時遇到權限問題：

1. 確保您的使用者具有讀寫 Nginx 設定檔案的必要權限
2. 對於某些操作，您可能需要以提升的權限執行 Nginx UI
3. 檢查檔案權限：
   ```bash
   # 檢查設定檔案權限
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # 檢查資料目錄權限
   ls -la $(brew --prefix)/var/nginx-ui/
   ```

### 服務無法啟動

如果服務啟動失敗：

1. **檢查服務狀態：**
   ```bash
   brew services list | grep nginx-ui
   ```

2. **驗證設定檔案是否存在且有效：**
   ```bash
   # 檢查設定檔案是否存在
   ls -la $(brew --prefix)/etc/nginx-ui/app.ini

   # 測試設定
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini --help
   ```

3. **嘗試手動執行以查看錯誤訊息：**
   ```bash
   nginx-ui serve -config $(brew --prefix)/etc/nginx-ui/app.ini
   ```

4. **檢查連接埠衝突：**
   ```bash
   # 檢查連接埠 9000 是否已被佔用
   lsof -i :9000

   # 檢查 HTTP 質詢連接埠是否被佔用
   lsof -i :9180
   ```

## 獲取幫助

如果遇到任何問題：

1. 查看 [官方文件](https://nginxui.com)
2. 在 [GitHub](https://github.com/0xJacky/nginx-ui/issues) 上搜尋現有問題
3. 如果您的問題尚未回報，請建立新問題

## 下一步

安裝完成後，您可以：

1. 存取 `http://localhost:9000` 的 Web 介面
2. 完成初始設定精靈
3. 開始設定您的 Nginx 站點
4. 探索 [設定指南](./config-server) 進行進階設定
