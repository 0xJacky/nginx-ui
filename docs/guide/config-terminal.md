# Terminal

## StartCmd

- Type: `string`
- Default: `login`
- Version: `>= v2.0.0-beta.37`

This option is used to set the start command of the web terminal.

::: warning
For security reason, we use `login` as the start command, so you have to log in via the default authentication method of
the Linux. If you don't want to enter your username and password for verification every time you access the web
terminal, please set it to `bash` or `zsh` (if installed).
:::
