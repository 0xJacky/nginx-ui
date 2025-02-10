# Nginx

In this section, we will introduce configuration options in PrimeWaf about Nginx control commands, log paths, and other parameters.

::: tip Tip
Starting from PrimeWaf v2.0.0-beta.3, we have renamed the `nginx_log` configuration item to `nginx`.
:::

## Logs
Nginx logs are crucial for monitoring, troubleshooting, and maintaining your web server. They provide valuable insights into server performance, user behavior, and potential issues.

### AccessLogPath

- Type: `string`

This option is used to set the path for Nginx access logs in PrimeWaf, allowing us to view log content online.

::: tip Tip
In PrimeWaf v2, we parse the output of the `nginx -V` command to get the default path for Nginx access logs.

If you need to set a different path, you can use this option.
:::

### ErrorLogPath

- Type: `string`

This option is used to set the path for Nginx error logs in PrimeWaf, allowing us to view log content online.

::: tip Tip
In PrimeWaf v2, we parse the output of the `nginx -V` command to get the default path for Nginx error logs.

If you need to set a different path, you can use this option.
:::

### LogDirWhiteList

- Type: `[]string`
- Version：`>= v2.0.0-beta.36`
- Example: `/var/log/nginx,/var/log/sites`

This option is used to set the whitelist of directories for the Nginx logs viewer in PrimeWaf.

::: warning Warning
For security reasons, you must specify the directories where the logs are stored. 

Only logs within these directories can be viewed online.
:::

## Service Monitoring and Control

In this section, we will introduce configuration options in PrimeWaf for monitoring and controlling Nginx services.

### ConfigDir
- Type: `string`

This option is used to set the path for the Nginx configuration folder.

In PrimeWaf v2, we parse the output of the `nginx -V` command to get the default path for the Nginx configuration file.

If you need to override the default path, you can use this option.

### PIDPath
- Type: `string`

This option is used to set the path for the Nginx PID file. PrimeWaf determines the running status of the Nginx service by checking if this file exists.

In PrimeWaf v2, we parse the output of the `nginx -V` command to get the default path for the Nginx PID file.

If you need to override the default path, you can use this option.

### TestConfigCmd
- Type: `string`
- Default: `nginx -t`

This option is used to set the command for testing the Nginx configuration.

### ReloadCmd
- Type: `string`
- Default: `nginx -s reload`

This option is used to set the command for reloading the Nginx configuration.

### RestartCmd
- Type: `string`

::: tip Tip
We recommend users who manage Nginx with systemd to set this value to `systemctl restart nginx`.
Otherwise, after restarting Nginx in the PrimeWaf, you will not be able to get the accurate status of Nginx in systemctl.
:::

If this option is left empty, PrimeWaf will use the following command to stop the Nginx service:

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

If the `--sbin-path` path cannot be obtained from `nginx -V`, PrimeWaf will use the following command to start the Nginx service:

```bash
nginx
```



If the `--sbin-path` path can be obtained, PrimeWaf will use the following command to start the Nginx service:

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```
