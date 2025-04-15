# 重設初始使用者密碼

`reset-password` 命令允許您將初始管理員賬戶的密碼重設為隨機生成的 12 位密碼，包含大寫字母、小寫字母、數字和特殊符號。

此功能在 `v2.0.0-rc.4` 版本中引入。

## 使用方法

要重設初始使用者的密碼，請執行：

```bash
nginx-ui reset-password --config=/path/to/app.ini
```

此命令將：
1. 生成一個安全的隨機密碼（12 個字元）
2. 重設初始使用者賬戶（使用者 ID 1）的密碼
3. 在應用程式日誌中輸出新密碼

## 參數

- `--config`：（必填）Nginx UI 設定檔的路徑

## 範例

```bash
# 使用預設設定檔位置重設密碼
nginx-ui reset-password --config=/path/to/app.ini

# 輸出將包含生成的密碼
2025-03-03 03:24:41     INFO    user/reset_password.go:52       confPath: ../app.ini
2025-03-03 03:24:41     INFO    user/reset_password.go:59       dbPath: ../database.db
2025-03-03 03:24:41     INFO    user/reset_password.go:92       User: root, Password: X&K^(X0m(E&&
```

## 設定檔位置

- 如果您使用 Linux 一鍵安裝指令碼安裝的 Nginx UI，設定檔位於：
  ```
  /usr/local/etc/nginx-ui/app.ini
  ```
  
  您可以直接使用以下命令：
  ```bash
  nginx-ui reset-password --config /usr/local/etc/nginx-ui/app.ini
  ```

## Docker 使用方法

如果您在 Docker 容器中執行 Nginx UI，需要使用 `docker exec` 命令：

```bash
docker exec -it <nginx-ui-container> nginx-ui reset-password --config=/etc/nginx-ui/app.ini
```

請將 `<nginx-ui-container>` 替換為您實際的容器名稱或 ID。

## 注意事項

- 如果您忘記了初始管理員密碼，此命令很有用
- 新密碼將顯示在日誌中，請確保立即複製它
- 您必須有權存取伺服器的命令列才能使用此功能
- 資料庫檔案必須存在才能使此命令正常工作 
