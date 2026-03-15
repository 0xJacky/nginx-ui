# Http

## GithubProxy
- 版本: `>= v2.0.0-beta.37`
- 类型：`string`
- 建议：`https://cloud.nginxui.com/`

- 对于可能在从 Github 下载资源时遇到困难的用户（如在中国大陆），此选项允许他们为 github.com 设置代理，以提高可访问性。

## InsecureSkipVerify

- 版本：`>= v2.0.0-beta.37`
- 类型: `bool`

此选项用于配置 Nginx UI 服务器在与其他服务器建立 TLS 连接时是否跳过证书验证。

## WebSocketTrustedOrigins

- 类型: `[]string`
- 默认值: 空
- 示例: `http://localhost:5173,https://admin.example.com`

此选项用于为已认证的 WebSocket 连接额外声明可信浏览器来源。

当 Nginx UI 通过带有不同公网域名的反向代理访问、需要同时支持多个管理域名，或本地开发时前后端运行在不同端口时，可以配置该选项。

请尽量保持列表最小化。对于同源的 WebSocket 请求，不需要额外加入这里。
