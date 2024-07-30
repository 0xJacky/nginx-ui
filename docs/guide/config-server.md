# Server

The server section of the Nginx UI configuration deals with various settings that control the behavior and operation of
the Nginx UI server. In this section, we will discuss the available options, their default values, and their purpose.

## HttpHost
- Type: `string`
- Default：`0.0.0.0`

The hostname on which the Nginx UI server listens for incoming HTTP requests.
Changing the default hostname can be useful for improving the security of Nginx UI.

## HttpPort

- Type: `int`
- Default: `9000`

Nginx UI server listen port. This option is used to configure the port on which the Nginx UI server listens for incoming
HTTP requests. Changing the default port can be useful for avoiding port conflicts or enhancing security.

## RunMode

- Type: `string`
- Supported value: `release`，`debug`
- Default: `debug`

This option is used to configure the running mode of the Nginx UI server, which mainly affects the level of log printing.

The log level of Nginx UI is divided into 6 levels: `Debug`, `Info`, `Warn`, `Error`, `Panic` and `Fatal`. These log levels increase in severity.

When using the `debug` mode, Nginx UI will print SQL and its execution time and caller on the console, and the log of `Debug` level or higher will also be printed.

When using the `release` mode, Nginx UI will not print the execution time and caller of SQL on the console, and only the log of `Info` level or higher will be printed.

## JwtSecret
- Type: `string`

This option is used to configure the key used by the Nginx UI server to generate JWT.

JWT is a standard for verifying user identity. It can generate a token after the user logs in, and then use the token to verify the user's identity in subsequent requests.

If you use the one-click installation script to deploy Nginx UI, the script will generate a UUID value and set it as the value of this option.

## HTTPChallengePort

- Type: `int`
- Default: `9180`

This option is used to set the port for backend listening in the HTTP01 challenge mode when obtaining Let's Encrypt
certificates. The HTTP01 challenge is a domain validation method used by Let's Encrypt to verify that you control the
domain for which you're requesting a certificate.

## Email
- Type: `string`

When obtaining a Let's Encrypt certificate, this option is used to set your email address.
Let's Encrypt will use your email address to notify you of the expiration date of your certificate.

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

For users who may experience difficulties downloading resources from GitHub (such as in mainland China), this option
allows them to set a proxy for github.com to improve accessibility.

## CertRenewalInterval

- Version：`>= v2.0.0-beta.22`
- Type: `int`
- Default value: `7`

This option is used to set the automatic renewal interval of the Let's Encrypt certificate.
By default, Nginx UI will automatically renew the certificate every 7 days.

## RecursiveNameservers

- Version：`>= v2.0.0-beta.22`
- Type: `[]string`
- Example: `8.8.8.8:53,1.1.1.1:53`

This option is used to set the recursive nameservers used by
Nginx UI in the DNS challenge step of applying for a certificate.
If this option is not configured, Nginx UI will use the nameservers settings of the operating system.

## SkipInstallation

- Version：`>= v2.0.0-beta.23`
- Type: `bool`
- Default value: `false`

You can skip the installation of the Nginx UI server by setting this option to `true`.
This is useful when you want to deploy Nginx UI to multiple servers with
a same configuration file or environment variables.

By default, if you enabled the skip installation mode without setting the `JWTSecret` and `NodeSecret` options
in the server section, Nginx UI will generate a random UUID value for these two options.

Plus, if you don't set the `Email` option also in the server section,
Nginx UI will not create a system initial acme user, this means you can't apply for an SSL certificate in this server.

## Name

- Version：`>= v2.0.0-beta.23`
- Type: `string`

Use this option to customize the name of local server to be displayed in the environment indicator.

## InsecureSkipVerify

- Version：`>= v2.0.0-beta.30`
- Type: `bool`

This option is used to skip the verification of the certificate of servers when Nginx UI sends requests to them.
