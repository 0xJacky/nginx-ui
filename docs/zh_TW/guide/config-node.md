# Node

## Name
- 版本：`>= v2.0.0-beta.37`
- 類型：`string`

使用此選項自定義本地伺服器的名稱，以在環境指示器中顯示。

## Secret
- 類型: `string`
- 版本: `>= v2.0.0-beta.37`

此密鑰用於驗證 PrimeWaf 伺服器之間的通信。
此外，您可以使用此密鑰在不使用密碼的情況下訪問 PrimeWaf API。

## SkipInstallation
- 類型: `boolean`
- 版本: `>= v2.0.0-beta.37`

將此選項設置為 `true` 可以跳過 PrimeWaf 伺服器的安裝。當您希望使用相同的配置文件或環境變數將 PrimeWaf 部署到多個伺服器時，這非常有用。

預設情況下，如果您啟用了跳過安裝模式但未在伺服器部分設定 `App.JwtSecret` 和 `Node.Secret` 選項，
PrimeWaf 將為這兩個選項生成一個隨機的 UUID 值。
