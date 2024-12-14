# Node

## Name
- 版本：`>= v2.0.0-beta.37`
- 类型：`string`

使用此选项自定义本地服务器的名称，以在环境指示器中显示。


## Secret
- 类型: `string`
- 版本: `>= v2.0.0-beta.37`

此密钥用于验证 Nginx UI 服务器之间的通信。
此外，您可以使用此密钥在不使用密码的情况下访问 Nginx UI API。

## SkipInstallation
- 类型: `boolean`
- 版本: `>= v2.0.0-beta.37`

通过将此选项设置为 `true`，您可以跳过 Nginx UI 服务器的安装。
当您希望使用相同的配置文件或环境变量将 Nginx UI 部署到多个服务器时，这非常有用。

默认情况下，如果您启用了跳过安装模式，而没有在服务器部分设置 `App.JwtSecret` 和 `Node.Secret` 选项，
Nginx UI 将为这两个选项生成一个随机的 UUID 值。
