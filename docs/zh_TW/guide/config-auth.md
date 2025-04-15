# Auth
從 v2.0.0-beta.26 版本開始，您可以在設定檔的 `auth` 部分設定授權選項。

## IPWhiteList
- 類型：`string`
- 範例：`10.0.0.1`

```ini
[auth]
IPWhiteList = 10.0.0.1
IPWhiteList = 10.0.0.2
IPWhiteList = 2001:0000:130F:0000:0000:09C0:876A:130B
```

預設情況下，如果您沒有設定 IPWhiteList，所有 IP 地址都允許存取 Nginx UI。
一旦您設定了 IPWhiteList，只有列表中和 `127.0.0.1` 的 IP 地址的使用者可以存取 Nginx UI，
其他人將收到 `403 Forbidden` 錯誤。

## BanThresholdMinutes
- Type: `int`
- Default: `10`

預設情況下，如果使用者在 10 分鐘內登入失敗 10 次，使用者將被禁止登入 10 分鐘。

## MaxAttempts
- Type: `int`
- Default: `10`

預設情況下，如果使用者在 10 分鐘內登入失敗 10 次，使用者將被禁止登入 10 分鐘。
