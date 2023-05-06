# Nginx 日志

Nginx 日志对于监控、排查问题和维护您的 Web 服务器至关重要。它们提供了有关服务器性能、用户行为和潜在问题的宝贵见解。在本节中，我们将讨论两种主要类型的日志：访问日志和错误日志。

对于从 v1.5.2 或更早版本升级的 Nginx-UI Docker 用户，在配置 `app.ini` 之前，至关重要的是在您的 `nginx.conf`
中添加单独的 `access_log` 和 `error_log` 指令。

在 Nginx-UI 容器中，`/var/log/nginx/access.log` 是一个指向 `/dev/stdout` 的符号链接，而 `/var/log/nginx/error.log`
是一个指向 `/dev/stderr` 的符号链接。这种设置允许您使用 `docker logs nginx-ui` 命令查看 Nginx 和 Nginx-UI 日志。然而，这两个设备不支持
`tail` 命令，因此有必要使用额外的日志文件来记录 Nginx 日志。

示例：

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

此外，在 `app.ini` 中设置 nginx 访问日志和错误日志路径，然后重新启动 nginx-ui。

示例：

```ini
[nginx_log]
AccessLogPath = /var/log/nginx/access.local.log
ErrorLogPath = /var/log/nginx/error.local.log
```

## AccessLogPath

- 类型：`string`

此选项用于为 Nginx UI 设置 Nginx 访问日志的路径，以便我们在线查看日志内容。

## ErrorLogPath

- 类型：`string`

此选项用于为 Nginx UI 设置 Nginx 错误日志的路径，以便我们在线查看日志内容。
