# Terminal

## StartCmd

- 類型: `string`
- 預設值: `login`
- 版本: `>= v2.0.0-beta.37`

此選項用於設置 Web 終端的啟動命令。

::: warning 警告
出於安全原因，我們將啟動命令設置為 `login`，因此您必須通過 Linux 的預設身份驗證方法登錄。
如果您不想每次訪問 Web 終端時都輸入用戶名和密碼進行驗證，請將其設置為 `bash` 或 `zsh`（如果已安裝）。
:::
