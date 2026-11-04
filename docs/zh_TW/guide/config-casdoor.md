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

## 帳戶對應

Casdoor 只能登入 Nginx UI 中已存在的使用者，不會自動建立帳戶。
只有當 Casdoor 傳回的存取權杖符合以下全部條件時，登入才會被接受：

- 由 `CertificatePath` 指定的憑證簽署；
- 是存取權杖，而不是重新整理權杖；
- 受眾（`aud`）等於 `ClientId`，對於共用的 Casdoor 應用程式則等於 `<ClientId>-org-<Organization>`；
- `owner` 宣告等於 `Organization`。其他組織（包括 `built-in`）的使用者即使能登入該 Casdoor 應用程式，也會被拒絕。

Nginx UI 使用者名稱必須與 Casdoor 使用者名稱（使用者的 `Name`，而不是顯示名稱）一致。使用者名稱所在的宣告取決於應用程式的權杖格式：

| 權杖格式 | 使用者名稱宣告 | 說明 |
|----------|----------------|------|
| `JWT`（預設） | `name` | |
| `JWT-Empty` | `name` | |
| `JWT-Standard` | `preferred_username` | `name` 是顯示名稱，不會被使用 |
| `JWT-Custom` | `name` | 需要在權杖欄位中同時選擇 `Owner` 和 `Name`，並且不要新增名為 `owner`、`name` 或 `preferred_username`（不區分大小寫）的權杖屬性或 `Properties` 欄位 |

不包含 `owner` 宣告的權杖會被拒絕，因為無法驗證其所屬組織。

當登入被拒絕或找不到對應使用者時，錯誤回應中會包含 `organization`、`resolved_by` 和 `resolved_username`，方便排查對應問題。

::: warning
任何能夠在 `Organization` 中註冊與 Nginx UI 使用者同名的 Casdoor 帳戶的人，都能以該使用者身分登入。請為 Nginx UI 使用的應用程式關閉註冊（以及訪客登入），或者確保每個 Nginx UI 使用者名稱在 Casdoor 中都已被對應的本人佔用。同時請將 `Name` 帳戶項目的修改規則保持為 `Admin`（Casdoor 預設值），防止使用者自行修改使用者名稱。
:::
