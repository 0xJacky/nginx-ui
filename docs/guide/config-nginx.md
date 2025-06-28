# Nginx

In this section, we will introduce configuration options in Nginx UI about Nginx control commands, log paths, and other parameters.

::: tip Tip
Starting from Nginx UI v2.0.0-beta.3, we have renamed the `nginx_log` configuration item to `nginx`.
:::

## Logs
Nginx logs are crucial for monitoring, troubleshooting, and maintaining your web server. They provide valuable insights into server performance, user behavior, and potential issues.

### AccessLogPath

- Type: `string`

This option is used to set the path for Nginx access logs in Nginx UI, allowing us to view log content online.

::: tip Tip
In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for Nginx access logs.

If you need to set a different path, you can use this option.
:::

### ErrorLogPath

- Type: `string`

This option is used to set the path for Nginx error logs in Nginx UI, allowing us to view log content online.

::: tip Tip
In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for Nginx error logs.

If you need to set a different path, you can use this option.
:::

### LogDirWhiteList

- Type: `[]string`
- Version：`>= v2.0.0-beta.36`
- Example: `/var/log/nginx,/var/log/sites`

This option is used to set the whitelist of directories for the Nginx logs viewer in Nginx UI.

::: warning Warning
For security reasons, you must specify the directories where the logs are stored. 

Only logs within these directories can be viewed online.
:::

## Service Monitoring and Control

In this section, we will introduce configuration options in Nginx UI for monitoring and controlling Nginx services.

### ConfigDir
- Type: `string`

This option is used to set the path for the Nginx configuration folder.

In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for the Nginx configuration file.

If you need to override the default path, you can use this option.

### PIDPath
- Type: `string`

This option is used to set the path for the Nginx PID file. Nginx UI determines the running status of the Nginx service by checking if this file exists.

In Nginx UI v2, we parse the output of the `nginx -V` command to get the default path for the Nginx PID file.

If you need to override the default path, you can use this option.

### SbinPath
- Type: `string`
- Version: `>= v2.1.10`

This option is used to set the path for the Nginx executable file.

By default, Nginx UI will try to find the Nginx executable file in `$PATH`.

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
Otherwise, after restarting Nginx in the Nginx UI, you will not be able to get the accurate status of Nginx in systemctl.
:::

If this option is left empty, Nginx UI will use the following command to stop the Nginx service:

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

If the `--sbin-path` path cannot be obtained from `nginx -V`, Nginx UI will use the following command to start the Nginx service:

```bash
nginx
```

If the `--sbin-path` path can be obtained, Nginx UI will use the following command to start the Nginx service:

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```

### StubStatusPort
- Type: `uint`
- Default: `51820`
- Version: `>= v2.0.0-rc.6`

This option is used to set the port for the Nginx stub status module. The stub status module provides basic status information about Nginx, which is used by Nginx UI to monitor the server's performance.

::: tip Tip
Make sure the port you set is not being used by other services.
:::

## Container Control

In this section, we will introduce configuration options in Nginx UI for controlling Nginx services running in another Docker container.

### ContainerName
- Type: `string`
- Version: `>= v2.0.0-rc.6`

This option is used to specify the name of the Docker container where Nginx is running.

If this option is empty, Nginx UI will control the Nginx service on the local machine or within the current container.

If this option is not empty, Nginx UI will control the Nginx service running in the specified container.

::: tip Tip
If you are using the official Nginx UI container and want to control Nginx in another container, you must map the host's docker.sock to the Nginx UI container.

For example: `-v /var/run/docker.sock:/var/run/docker.sock`
:::
