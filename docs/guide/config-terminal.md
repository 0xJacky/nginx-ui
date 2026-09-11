# Terminal

## StartCmd

- Type: `string`
- Default: `login` on Linux and macOS; `cmd.exe` on Windows
- Version: `>= v2.0.0-beta.37`

This option is used to set the start command of the web terminal.

::: warning
For security reason, we use `login` as the start command, so you have to log in via the default authentication method of
the Linux. If you don't want to enter your username and password for verification every time you access the web
terminal, please set it to `bash` or `zsh` (if installed).
:::

Windows installations default to `cmd.exe` so the terminal remains available
when no explicit `StartCmd` is present. Set `StartCmd` to `powershell.exe` if
PowerShell is preferred.
