# Server

The server section of the Nginx UI configuration deals with various settings that control the behavior and operation of
the Nginx UI server. In this section, we will discuss the available options, their default values, and their purpose.

## HttpPort

- Type: `int`
- Default: `9000`

Nginx UI server listen port. This option is used to configure the port on which the Nginx UI server listens for incoming
HTTP requests. Changing the default port can be useful for avoiding port conflicts or enhancing security.

## RunMode

- Type: `string`
- Supported value: `release`, `debug`

::: tip
At present, we have not yet adapted to this option, and there will be no significant differences between release and
debug in terms of usage.
:::

## HTTPChallengePort

- Type: `int`
- Default: `9180`

This option is used to set the port for backend listening in the HTTP01 challenge mode when obtaining Let's Encrypt
certificates. The HTTP01 challenge is a domain validation method used by Let's Encrypt to verify that you control the
domain for which you're requesting a certificate.

## Database

- Type: `string`
- Default: `database`

This option is used to set the name of the sqlite database used by Nginx UI to store its data.

## StartCmd

- Type: `string`
- Default: `login`

This option is used to set the start command of the web terminal.

::: warning
For security reason, we use `login` as the start command, so you have to log in via the default authentication method of
the Linux. If you don't want to enter your username and password for verification every time you access the web
terminal, please set it to `bash` or `zsh` (if installed).
:::

## PageSize

- Type: `int`
- Default: 10

This option is used to set the page size of list pagination in the Nginx UI. Adjusting the page size can help in
managing large amounts of data more effectively, but a too large number can increase the load on the server.

## CADir

- Type: `string`

When applying for a Let's Encrypt certificate, we use the default CA address of Let's Encrypt. If you need to debug or
obtain certificates from other providers, you can set CADir to their address.

::: tip
Please note that the address provided by
CADir needs to comply with the `RFC 8555` standard.
:::

## GithubProxy

- Type: `string`
- Suggestion: `https://mirror.ghproxy.com/`

For users who may experience difficulties downloading resources from Github (such as in mainland China), this option
allows them to set a proxy for github.com to improve accessibility.
