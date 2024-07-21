# Auth
从 v2.0.0-beta.26 版本开始，您可以在配置文件的 `auth` 部分设置授权选项。

## IPWhiteList
- 类型：`string`
- 示例：`10.0.0.1`

```ini
[auth]
IPWhiteList = 10.0.0.1
IPWhiteList = 10.0.0.2
IPWhiteList = 2001:0000:130F:0000:0000:09C0:876A:130B
```

默认情况下，如果您没有设置 `IPWhiteList`，所有 IP 地址都允许访问 Nginx UI。

一旦您设置了 `IPWhiteList`，只有列表中和 `127.0.0.1` 的 IP 地址的用户可以访问 Nginx UI，
其他人将收到 `403 Forbidden` 错误。

## BanThresholdMinutes
- Type: `int`
- Default: `10`

默认情况下，如果用户在 10 分钟内登录失败 10 次，用户将被禁止登录 10 分钟。

## MaxAttempts
- Type: `int`
- Default: `10`

默认情况下，用户可以在 10 分钟内尝试登录 10 次。
