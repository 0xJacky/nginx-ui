# Terminal

## StartCmd

- 类型: `string`
- 默认值: `login`
- 版本: `>= v2.0.0-beta.37`

此选项用于设置 Web 终端的启动命令。

::: warning 警告
出于安全原因，我们将启动命令设置为 `login`，因此您必须通过 Linux 的默认身份验证方法登录。
如果您不想每次访问 Web 终端时都输入用户名和密码进行验证，请将其设置为 `bash` 或 `zsh`（如果已安装）。
:::
