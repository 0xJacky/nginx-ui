# Casdoor
本节介绍如何配置 Casdoor 作为 Nginx UI 的身份验证提供程序，该功能由 @Jraaay 贡献。

Casdoor 是一个强大的、全面的身份认证解决方案，支持 OAuth 2.0、SAML 2.0、LDAP、AD 和多种社交登录方式。通过集成 Casdoor，Nginx UI 可以利用这些功能来提升安全性和用户体验。

## Endpoint
- 类型：`string`

这是 Casdoor 服务器的 URL。您需要确保 Nginx UI 可以访问此 URL。

## ExternalUrl
- 类型：`string`
- 版本: `>= v2.0.0-beta.42`

这是 Casdoor 服务器的外部 URL。它用于生成重定向 URI，在不配置次选项的情况下，将使用 Endpoint 作为重定向 URI 的基本 URL。

## ClientId
- 类型：`string`

这是 Casdoor 为您的应用生成的客户端 ID。它用于在身份验证过程中标识您的应用。

## ClientSecret
- 类型：`string`

这是 Casdoor 为您的应用生成的客户端密钥。它是保持您的应用安全所必需的。

## CertificatePath
- 类型：`string`

这是用于身份验证过程中的证书的路径。确保它是有效和可信的。

## Organization
- 类型：`string`

这是您在 Casdoor 中设置的组织名称。Casdoor 将使用此信息来处理身份验证请求。

## Application
- 类型：`string`

这是您在 Casdoor 中创建的应用名称。

## RedirectUri
- 类型：`string`

这是用户在成功登录或授权后重定向到的 URI。它应与 Casdoor 应用配置中的重定向 URI 一致。

## 账户映射

Casdoor 只能登录 Nginx UI 中已存在的用户，不会自动创建账户。
只有当 Casdoor 返回的访问令牌满足以下全部条件时，登录才会被接受：

- 由 `CertificatePath` 指定的证书签名；
- 是访问令牌，而不是刷新令牌；
- 受众（`aud`）等于 `ClientId`，对于共享的 Casdoor 应用则等于 `<ClientId>-org-<Organization>`；
- `owner` 声明等于 `Organization`。其他组织（包括 `built-in`）的用户即使能登录该 Casdoor 应用，也会被拒绝。

Nginx UI 用户名必须与 Casdoor 用户名（用户的 `Name`，而不是显示名称）一致。用户名所在的声明取决于应用的令牌格式：

| 令牌格式 | 用户名声明 | 说明 |
|----------|------------|------|
| `JWT`（默认） | `name` | |
| `JWT-Empty` | `name` | |
| `JWT-Standard` | `preferred_username` | `name` 是显示名称，不会被使用 |
| `JWT-Custom` | `name` | 需要在令牌字段中同时选择 `Owner` 和 `Name`，并且不要添加名为 `owner`、`name` 或 `preferred_username`（不区分大小写）的令牌属性或 `Properties` 字段 |

不包含 `owner` 声明的令牌会被拒绝，因为无法校验其所属组织。

当登录被拒绝或找不到对应用户时，错误响应中会包含 `organization`、`resolved_by` 和 `resolved_username`，便于排查映射问题。

::: warning
任何能够在 `Organization` 中注册与 Nginx UI 用户同名的 Casdoor 账户的人，都能以该用户身份登录。请为 Nginx UI 使用的应用关闭注册（以及访客登录），或者确保每个 Nginx UI 用户名在 Casdoor 中都已被对应的本人占用。同时请将 `Name` 账户项的修改规则保持为 `Admin`（Casdoor 默认值），防止用户自行修改用户名。
:::
