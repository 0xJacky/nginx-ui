# Nginx Log

Nginx logs are essential for monitoring, troubleshooting, and maintaining your web server. They provide valuable
insights into server performance, user behavior, and potential issues. In this section, we will discuss the two primary
types of logs: access logs and error logs.

For Nginx-UI Docker users who are upgrading from v1.5.2 or earlier versions, it is crucial to add separate `access_log`
and `error_log` directives in your `nginx.conf` before configuring the `app.ini`.

In the Nginx-UI container, `/var/log/nginx/access.log` is a symlink pointing to `/dev/stdout`,
and `/var/log/nginx/error.log`
is a symlink pointing to `/dev/stderr`. This setup allows you to view both the Nginx and Nginx-UI logs using the `docker
logs nginx-ui` command. However, these two devices do not support the tail command, so it is necessary to use additional
log files to record Nginx logs.

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

Additionally, set nginx access log and error log path in `app.ini` and restart nginx-ui.

Example:

```ini
[nginx_log]
AccessLogPath = /var/log/nginx/access.local.log
ErrorLogPath = /var/log/nginx/error.local.log
```

## AccessLogPath

- Type: `string`

This option is used to set the path of nginx access log for Nginx UI, so we can view the log content online.

## ErrorLogPath

- Type: `string`

This option is used to set the path of nginx error log for Nginx UI, so we can view the log content online.
