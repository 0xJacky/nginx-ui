# Nginx

In this section, we will introduce configuration options in Nginx UI about Nginx control commands, log paths, and other parameters.

::: tip Tip
Starting from Nginx UI v2.0.0-beta.3, we have renamed the `nginx_log` configuration item to `nginx`.
:::

## Logs
Nginx logs are crucial for monitoring, troubleshooting, and maintaining your web server. They provide valuable insights into server performance, user behavior, and potential issues.

In this section, we will discuss two main types of logs: access logs and error logs.

For Nginx-UI Docker users upgrading from version v1.5.2 or earlier, you need to add separate `access_log` and `error_log` directives in `nginx.conf` before configuring `app.ini`.

In the Nginx-UI container, `/var/log/nginx/access.log` is a symbolic link to `/dev/stdout`, and `/var/log/nginx/error.log` is a symbolic link to `/dev/stderr`. This setup allows you to view Nginx and Nginx-UI logs using the `docker logs nginx-ui` command. However, these devices do not support the `tail` command, so it is necessary to use additional log files to record Nginx logs.

Example:

```nginx
error_log /var/log/nginx/error.log notice;
error_log /var/log/nginx/error.local.log notice;

http {
...
    access_log /var/log/nginx/access.log main;
    access_log /var/log/nginx/access.local.log main;
...
}
```

Afterward, set the nginx access log and error log paths in `app.ini`, then restart nginx-ui.

Example:

```ini
[nginx]
AccessLogPath = /var/log/nginx/access.local.log
ErrorLogPath = /var/log/nginx/error.local.log
```

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
