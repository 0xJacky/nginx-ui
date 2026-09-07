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
| `api:read` | 管理 API 的 `GET`、`HEAD`、`OPTIONS` 请求 |
| `api:write` | 管理 API 的变更请求，同时包含 API 读权限 |

MCP 权限与 API 权限彼此独立。授予 `mcp:write` 不会获得管理 API 访问权限，授予 `api:write` 也不会获得 MCP 访问权限。

服务令牌无法访问交互式账户安全操作、受保护设置、其他服务令牌管理接口，也无法打开 Web 终端。
这些操作请使用已认证的管理员会话。

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

列出并创建用户：

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users list
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users create \
  --name deploy-user --password-file /run/secrets/deploy-user-password
```

在跳过安装流程的部署中，预置用户环境变量仍可用于初始化首个用户。
实例启动后新增用户请使用 `ctl users create`。

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
