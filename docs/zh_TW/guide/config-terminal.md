# Terminal

## StartCmd

- 類型：`string`
- 預設值：`login`
- 版本：`>= v2.0.0-beta.37`

此選項用於設定 Web 終端的啟動命令。

::: warning 警告
出於安全原因，我們將啟動命令設定為 `login`，因此您必須透過 Linux 的預設身份驗證方法登入。
如果您不想每次存取 Web 終端時都輸入使用者名稱和密碼進行驗證，請將其設定為 `bash` 或 `zsh`（如果已安裝）。
:::
