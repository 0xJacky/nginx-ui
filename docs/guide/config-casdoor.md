# Casdoor
本节介绍如何配置 Casdoor 作为 Nginx UI 的身份验证提供程序，该功能由 @Jraaay 贡献。

Casdoor 是一个强大的、全面的身份认证解决方案，支持 OAuth 2.0、SAML 2.0、LDAP、AD 和多种社交登录方式。通过集成 Casdoor，Nginx UI 可以利用这些功能来提升安全性和用户体验。

## Endpoint
- 类型：`string`

这是 Casdoor 服务器的 URL。您需要确保 Nginx UI 可以访问此 URL。

## ClientId
- 类型：`string`

这是 Casdoor 为您的应用生成的客户端 ID。它用于在身份验证过程中标识您的应用。

## ClientSecret
- 类型：`string`

这是 Casdoor 为您的应用生成的客户端密钥。它是保持您的应用安全所必需的。

## Certificate
- 类型：`string`

这是用于身份验证过程中的证书。确保它是有效和可信的。

## Organization
- 类型：`string`

这是您在 Casdoor 中设置的组织名称。Casdoor 将使用此信息来处理身份验证请求。

## Application
- 类型：`string`

这是您在 Casdoor 中创建的应用名称。

## RedirectUri
- 类型：`string`

这是用户在成功登录或授权后重定向到的 URI。它应与 Casdoor 应用配置中的重定向 URI 一致。
