# Auth
從 v2.0.0-beta.26 版本開始，您可以在配置文件的 `auth` 部分設置授權選項。

## IPWhiteList
- 類型：`string`
- 範例：`10.0.0.1`

```ini
[auth]
IPWhiteList = 10.0.0.1
IPWhiteList = 10.0.0.2
IPWhiteList = 2001:0000:130F:0000:0000:09C0:876A:130B
```

默認情況下，如果您沒有設置 IPWhiteList，所有 IP 地址都允許訪問 PrimeWaf。
一旦您設置了 IPWhiteList，只有列表中和 `127.0.0.1` 的 IP 地址的用戶可以訪問 PrimeWaf，
其他人將收到 `403 Forbidden` 錯誤。

## BanThresholdMinutes
- Type: `int`
- Default: `10`

默認情況下，如果用戶在 10 分鐘內登錄失敗 10 次，用戶將被禁止登錄 10 分鐘。

## MaxAttempts
- Type: `int`
- Default: `10`

默認情況下，如果用戶在 10 分鐘內登錄失敗 10 次，用戶將被禁止登錄 10 分鐘。
