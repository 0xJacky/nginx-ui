# Casdoor
本節介紹如何設定 Casdoor 作為 Nginx UI 的身份驗證提供程式，該功能由 @Jraaay 貢獻。

Casdoor 是一個強大的、全面的身份認證解決方案，支援 OAuth 2.0、SAML 2.0、LDAP、AD 和多種社交登入方式。透過整合 Casdoor，Nginx UI 可以利用這些功能來提升安全性和使用者體驗。

## Endpoint
- 類型：`string`

這是 Casdoor 伺服器的 URL。您需要確保 Nginx UI 可以存取此 URL。

## ExternalUrl
- 種類：`string`
- 版本：`>= v2.0.0-beta.42`

這是 Casdoor 伺服器的外部 URL。它用於生成重導向 URI，在未設定此選項的情況下，將使用 Endpoint 作為重導向 URI 的基本 URL。

## ClientId
- 類型：`string`

這是 Casdoor 為您的應用程式生成的客戶端 ID。它用於在身份驗證過程中標識您的應用程式。

## ClientSecret
- 類型：`string`

這是 Casdoor 為您的應用程式生成的客戶端金鑰。它是保持您的應用程式安全所必需的。

## Certificate
- 類型：`string`

這是用於身份驗證過程中的證書的路徑。確保它是有效和可信的。

## Organization
- 類型：`string`

這是您在 Casdoor 中設定的組織名稱。Casdoor 將使用此資訊來處理身份驗證請求。

## Application
- 類型：`string`

這是您在 Casdoor 中建立的應用程式名稱。

## RedirectUri
- 類型：`string`

這是使用者在成功登入或授權後重導向到的 URI。它應與 Casdoor 應用程式設定中的重導向 URI 一致。
