# Webauthn

Webauthn 是一种用于安全身份验证的网络标准。它允许用户使用生物识别、移动设备和 FIDO 安全密钥登录网站。

Webauthn 是一种无密码的身份验证方法，提供了比传统密码更安全、易用的替代方案。

从 `v2.0.0-beta.34` 版本开始，Nginx UI 支持将 Webauthn Passkey 作为登录和双因素认证（2FA）方法。

## Passkey

Passkey 是使用触摸、面部识别、设备密码或 PIN 验证您身份的 Webauthn 凭证。它们可用作密码替代品或作为 2FA 方法。

## 配置

为确保安全性，不能通过 UI 添加 Webauthn 配置。

请在 app.ini 配置文件中手动添加以下内容，并重新启动 Nginx UI。

### RPDisplayName

- 类型：`string`

  用于在注册新凭证时设置依赖方（RP）的显示名称。

### RPID

- 类型：`string`

  用于在注册新凭证时设置依赖方（RP）的 ID。

### RPOrigins

- 类型：`[]string`

  用于在注册新凭证时设置依赖方（RP）的来源（origins）。

完成后，刷新此页面并再次点击添加 Passkey。

由于某些浏览器的安全策略，除非在 `localhost` 上运行，否则无法在非 HTTPS 网站上使用 Passkey。

## 详细说明

1. **使用 Passkey 的自动 2FA：**

   当您使用 Passkey 登录时，所有后续需要 2FA 的操作将自动使用 Passkey。这意味着您无需在 2FA 对话框中手动点击 “通过 Passkey 进行认证”。

2. **删除 Passkey：**

   如果您使用 Passkey 登录后，前往“设置 > 认证”并删除当前的 Passkey，那么在当前会话中，Passkey 将不再用于后续的 2FA 验证。如果已配置基于时间的一次性密码（TOTP），则将改为使用它；如果未配置，则将关闭 2FA。

3. **添加新 Passkey：**

   如果您在未使用 Passkey 的情况下登录，然后通过 “设置 > 认证” 添加新的 Passkey，那么在当前会话中，新增的 Passkey 将优先用于后续所有的 2FA 验证。
