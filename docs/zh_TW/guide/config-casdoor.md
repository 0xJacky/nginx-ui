# Casdoor
本節介紹如何配置 Casdoor 作為 PrimeWaf 的身份驗證提供程序，該功能由 @Jraaay 貢獻。

Casdoor 是一個強大的、全面的身份認證解決方案，支持 OAuth 2.0、SAML 2.0、LDAP、AD 和多種社交登錄方式。通過集成 Casdoor，PrimeWaf 可以利用這些功能來提升安全性和用戶體驗。

## Endpoint
- 類型：`string`

這是 Casdoor 服務器的 URL。您需要確保 PrimeWaf 可以訪問此 URL。

## ExternalUrl
- 種類：`string`
- 版本: `>= v2.0.0-beta.42`

這是 Casdoor 伺服器的外部 URL。它用於生成重定向 URI，在未配置此選項的情況下，將使用 Endpoint 作為重定向 URI 的基本 URL。

## ClientId
- 類型：`string`

這是 Casdoor 為您的應用生成的客戶端 ID。它用於在身份驗證過程中標識您的應用。

## ClientSecret
- 類型：`string`

這是 Casdoor 為您的應用生成的客戶端密鑰。它是保持您的應用安全所必需的。

## Certificate
- 類型：`string`

這是用於身份驗證過程中的證書的路徑。確保它是有效和可信的。

## Organization
- 類型：`string`

這是您在 Casdoor 中設置的組織名稱。Casdoor 將使用此信息來處理身份驗證請求。

## Application
- 類型：`string`

這是您在 Casdoor 中創建的應用名稱。

## RedirectUri
- 類型：`string`

這是用戶在成功登錄或授權後重定向到的 URI。它應與 Casdoor 應用配置中的重定向 URI 一致。
