# Http

## GithubProxy
- 版本：`>= v2.0.0-beta.37`
- 類型：`string`
- 建議：`https://cloud.nginxui.com/`

對於可能在從 Github 下載資源時遇到困難的使用者（如在中國大陸），此選項允許他們為 github.com 設定代理，以提高可存取性。

## InsecureSkipVerify

- 版本：`>= v2.0.0-beta.37`
- 類型：`bool`

此選項用於設定 Nginx UI 伺服器在與其他伺服器建立 TLS 連接時是否跳過證書驗證。

## WebSocketTrustedOrigins

- 類型：`[]string`
- 預設值：空
- 範例：`http://localhost:5173,https://admin.example.com`

::: tip
由於 Nginx UI 現在使用基於票據（ticket-based）的 WebSocket 驗證方案，大多數部署情境**不再需要**設定此選項。
WebSocket 安全性透過要求 URL 查詢參數中攜帶顯式的短令牌來保障，該短令牌只能透過受 CSRF 保護的 API 端點取得。
此選項作為可選的縱深防禦措施保留。
:::

此選項用於為已驗證的 WebSocket 連線額外宣告可信任的瀏覽器來源。

當 Nginx UI 透過具有不同公開網域的反向代理存取、需要同時支援多個管理網域，或在本機開發時前後端執行於不同連接埠時，可以設定此選項。

請盡量將此清單保持在最小範圍。對於同源的 WebSocket 請求，不需要額外加入這裡。
