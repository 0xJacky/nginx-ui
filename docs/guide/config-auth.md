# Auth
From v2.0.0-beta.26, you can authorization settings in the `auth` section of the configuration file.

## IPWhiteList
- Type: `string`
- Example: `10.0.0.1`

```ini
[auth]
IPWhiteList = 10.0.0.1
IPWhiteList = 10.0.0.2
IPWhiteList = 2001:0000:130F:0000:0000:09C0:876A:130B
```

By default, if you do not set the `IPWhiteList`, all IP addresses are allowed to access the Nginx UI.

Once you set the `IPWhiteList`, only the users from IP addresses in the list and `127.0.0.1` can access the Nginx UI,
others will receive a `403 Forbidden` error.

## BanThresholdMinutes
- Type: `int`
- Default: `10`

By default, if a user fails to log in 10 times within 10 minutes, the user will be banned for 10 minutes.

## MaxAttempts
- Type: `int`
- Default: `10`

By default, a user can try to log in 10 times within 10 minutes.
