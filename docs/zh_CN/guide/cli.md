# 命令行接口

`nginx-ui ctl` 命令通过管理 API 管理正在运行的 Nginx UI 实例。
它适用于基础设施即代码、配置即代码、部署自动化以及远程运维。

## 创建访问令牌

1. 以管理员身份登录 Nginx UI。
2. 打开 **Preferences > Access Tokens**。
3. 按最小权限原则创建令牌，并设置过期时间。
4. 立即复制令牌。Nginx UI 不会再次显示该令牌。

API 权限范围如下：

| Scope | 访问权限 |
| --- | --- |
| `api:read` | 管理 API 的 `GET`、`HEAD`、`OPTIONS` 请求，已存储的密钥会被隐藏 |
| `api:write` | 管理 API 的变更请求，同时包含 API 读权限，可读取除插件表单值以外的已存储密钥 |

`api:read` 是只读角色，永远不会收到已存储的密钥。响应保持原有结构和所有非敏感字段，敏感值会替换为 `__NGINX_UI_REDACTED__`。范围包括通知渠道配置值、站点健康检查的请求头值、请求体以及探测与点击 URL 中的密码和查询参数值、DNS 域名中附带的 DNS 服务商凭据，以及证书详情中的私钥。DNS 凭据、ACME 账户和自动备份接口对所有服务令牌都会省略凭据字段，包括 S3 密钥和插件存储后端的表单值（`storage_config`）。站点健康检查中插件探测类型的表单值（`probe_config`）同样会对所有服务令牌（包括 `api:write`）替换为 `__NGINX_UI_REDACTED__`，因为该表单由插件定义，其中任何值都可能是凭据。保存健康检查时原样提交读取到的 `probe_config`（每个值仍为占位符）或省略 `probe_config`，只要探测类型和探测目标 URL 不变，就会保留已存储的值。如需修改探测类型、探测目标或任何一个值，请提交全部值；占位符与其他值混合提交会被拒绝。这只会让这些值不出现在 API 响应中，并不限制 `api:write` 令牌的其他能力，例如导出备份。通过 `GET /api/config` 读取敏感文件会返回 `403` 和错误码 `40019`，包括 `.key` 和 `.htpasswd` 文件、任何包含 PEM 或 PGP 私钥块的文件，以及 Nginx 配置在 `ssl_certificate_key`、`ssl_password_file`、`ssl_key_log`（含 `proxy_`、`grpc_`、`uwsgi_` 等模块前缀的变体）、`ssl_session_ticket_key`、`ssl_ech_file`、`quic_host_key`、`auth_basic_user_file` 或 `auth_jwt_key_file` 中引用的文件，配置按 Nginx 自身的读取方式解析，并跟随每一条 `include`；这些文件在配置历史中的版本也会被隐藏。证书和其他配置文件仍可读取。通过 `X-Node-ID` 订阅远程节点的站点导航 WebSocket 会返回 `403` 和错误码 `40301`，因为控制端无法对转发的帧进行脱敏，请改用 `GET /api/site_navigation`。如果自动化任务需要导出私钥或读取其他已存储的凭据，请使用 `api:write` 令牌。

MCP 权限与 API 权限彼此独立。授予 `mcp:write` 不会获得管理 API 访问权限，授予 `api:write` 也不会获得 MCP 访问权限。

服务令牌无法访问交互式账户安全操作、创建或修改交互式用户、受保护设置、其他服务令牌管理接口，也无法打开 Web 终端。
`api:read` 权限可以列出和查看用户，但用户的创建、修改、删除和恢复需要已认证的交互式管理员会话。

## 配置客户端

设置访问端点，并将令牌保存在仅自动化账号可读的文件中：

```bash
export NGINX_UI_CTL_ENDPOINT=https://nginx-ui.example.com
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users list
```

端点也可通过 `--endpoint` 或 `NGINX_UI_CTL_ENDPOINT` 提供。
令牌可通过 `NGINX_UI_CTL_TOKEN`、`--token-file` 或 `--token-stdin` 提供。
建议优先使用密钥文件或标准输入，避免令牌出现在命令历史或进程参数中。

如果使用私有 CA 证书，请通过 `--ca-file` 传入 PEM 证书链。
使用 `--node-id` 可将支持的请求路由到指定集群节点。

## 常见操作

使用 `api:read` 服务令牌列出用户：

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users list
```

使用交互式管理员令牌创建用户：

```bash
nginx-ui ctl --token-file /run/secrets/admin-session-token users create \
  --name deploy-user --password-file /run/secrets/deploy-user-password
```

在跳过安装流程的部署中，预置用户环境变量仍可用于初始化首个用户。
安装完成后，请通过 Web 界面或使用交互式管理员令牌执行 `ctl users create` 来新增用户。
已启用双因素认证的管理员应使用 Web 界面，以便完成安全会话验证。

列出证书，并注册 Nginx UI 服务器上已存在的证书文件：

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token certificates list
nginx-ui ctl --token-file /run/secrets/nginx-ui-token certificates import \
  --name example.com \
  --cert /etc/nginx/ssl/example.com/fullchain.pem \
  --key /etc/nginx/ssl/example.com/privkey.pem
```

专用证书命令会从输出中移除证书与私钥 PEM 内容，避免 CI 日志泄露密钥材料。

查看和控制 Nginx：

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx status
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx test
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx reload
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx restart
```

## 调用任意管理 API

通用 `api` 子命令可覆盖尚未提供专用命令的管理操作：

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token api sites?page=1
nginx-ui ctl --token-file /run/secrets/nginx-ui-token api \
  --method POST --data-file site.json sites
```

路径会解析到 `/api` 之下。
查询字符串会被保留，但会拒绝绝对 URL，以防凭据被重定向到其他主机。
请求体和响应体大小限制为 16 MiB。

## 令牌生命周期

创建、轮换和吊销服务令牌需要交互式管理员令牌（包括所需的二次验证）：

```bash
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens list
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens create \
  --name ci --scope api:write --expires-at 2027-01-01T00:00:00Z
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens rotate TOKEN_ID
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens revoke TOKEN_ID
```

轮换会立即使旧令牌失效。吊销为永久操作。
