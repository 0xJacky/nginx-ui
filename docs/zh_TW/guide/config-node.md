# Node

## Name
- 版本：`>= v2.0.0-beta.37`
- 類型：`string`

使用此選項自定義本機伺服器的名稱，以在環境指示器中顯示。

## Secret
- 類型：`string`
- 版本：`>= v2.0.0-beta.37`

此金鑰用於驗證 Nginx UI 伺服器之間的通訊。
此外，您可以使用此金鑰在不使用密碼的情況下存取 Nginx UI API。

## SkipInstallation
- 類型：`boolean`
- 版本：`>= v2.0.0-beta.37`

將此選項設定為 `true` 可以跳過 Nginx UI 伺服器的安裝。當您希望使用相同的設定檔或環境變數將 Nginx UI 部署到多個伺服器時，這非常有用。

預設情況下，如果您啟用了跳過安裝模式但未在伺服器部分設定 `App.JwtSecret` 和 `Node.Secret` 選項，
Nginx UI 將為這兩個選項生成一個隨機的 UUID 值。
