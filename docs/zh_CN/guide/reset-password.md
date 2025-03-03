# 重置初始用户密码

`reset-password` 命令允许您将初始管理员账户的密码重置为随机生成的12位密码，包含大写字母、小写字母、数字和特殊符号。

此功能在 `v2.0.0-rc.4` 版本中引入。

## 使用方法

要重置初始用户的密码，请运行：

```bash
nginx-ui reset-password --config=/path/to/app.ini
```

此命令将：
1. 生成一个安全的随机密码（12个字符）
2. 重置初始用户账户（用户ID 1）的密码
3. 在应用程序日志中输出新密码

## 参数

- `--config`：（必填）Nginx UI 配置文件的路径

## 示例

```bash
# 使用默认配置文件位置重置密码
nginx-ui reset-password --config=/path/to/app.ini

# 输出将包含生成的密码
2025-03-03 03:24:41     INFO    user/reset_password.go:52       confPath: ../app.ini
2025-03-03 03:24:41     INFO    user/reset_password.go:59       dbPath: ../database.db
2025-03-03 03:24:41     INFO    user/reset_password.go:92       User: root, Password: X&K^(X0m(E&&
```

## 配置文件位置

- 如果您使用 Linux 一键安装脚本安装的 Nginx UI，配置文件位于：
  ```
  /usr/local/etc/nginx-ui/app.ini
  ```
  
  您可以直接使用以下命令：
  ```bash
  nginx-ui reset-password --config /usr/local/etc/nginx-ui/app.ini
  ```

## Docker 使用方法

如果您在 Docker 容器中运行 Nginx UI，需要使用 `docker exec` 命令：

```bash
docker exec -it <nginx-ui-container> nginx-ui reset-password --config=/etc/nginx-ui/app.ini
```

请将 `<nginx-ui-container>` 替换为您实际的容器名称或 ID。

## 注意事项

- 如果您忘记了初始管理员密码，此命令很有用
- 新密码将显示在日志中，请确保立即复制它
- 您必须有权访问服务器的命令行才能使用此功能
- 数据库文件必须存在才能使此命令正常工作 