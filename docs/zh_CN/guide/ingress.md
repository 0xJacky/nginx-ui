# Ingress API

**Ingress** 是由外部控制器声明、由 Nginx UI 负责维持的反向代理入口。
控制器发送期望状态，Nginx UI 渲染 nginx 配置、签发证书、测试并重载 nginx，然后上报结果。
以这种方式生成的网站会被标记为受管理，不应手动编辑：下一次调和会重写它们。

典型用途：控制平面通过隧道（例如绑定在 `127.0.0.1` 上的 frp 远程端口）把服务接入本机，并在 443 端口上以公网主机名对外提供。

## 认证

所有端点都位于 `/api` 之下，并接受常规凭据：

- 签名的节点请求（`Signature` / `Signature-Input`）或旧版 `X-Node-Secret` 请求头，推荐控制器使用；
- 带有 `api:read` / `api:write` 权限范围的服务令牌。

写入端点需要安全会话；节点请求和 `api:write` 令牌均满足该要求。
`owner` 字段用于防止两个控制器互相覆盖，不是授权边界。

## 资源

```json
{
  "id": 7,
  "name": "tm-prod-insure-ocr",
  "owner": "telemetry-prod",
  "external_id": "mac-service:42",
  "labels": { "service": "insure-ocr" },
  "namespace_id": 0,
  "generation": 3,
  "site_path": "ingress-tm-prod-insure-ocr",
  "spec": {
    "hosts": ["ocr.example.com"],
    "backends": [
      {
        "name": "app",
        "lb": "round_robin",
        "keepalive": 16,
        "servers": [
          { "address": "127.0.0.1:24001", "weight": 1, "max_fails": 2, "fail_timeout": "10s", "down": false, "backup": false },
          { "address": "127.0.0.1:24002", "down": true, "backup": false }
        ]
      }
    ],
    "routes": [
      {
        "path_prefix": "/",
        "backend": "app",
        "websocket": true,
        "read_timeout": "300s",
        "send_timeout": "300s",
        "client_max_body_size": "64m",
        "allow_cidrs": []
      }
    ],
    "tls": {
      "mode": "acme",
      "cert_id": 0,
      "key_type": "2048",
      "challenge": "http01",
      "dns_credential_id": 0,
      "acme_user_id": 0,
      "redirect_https": true,
      "hsts": false
    },
    "policy": {
      "allow_cidrs": ["203.0.113.0/24"],
      "rate_limit": { "rps": 20, "burst": 40 },
      "client_max_body_size": "16m",
      "extra_headers": [{ "name": "X-Request-Source", "value": "ingress" }]
    }
  },
  "status": {
    "observed_generation": 3,
    "phase": "Ready",
    "conditions": [
      { "type": "ConfigApplied", "status": true, "reason": "Applied", "last_transition_time": "2026-10-15T08:00:00Z" },
      { "type": "CertReady", "status": true, "reason": "Issued", "last_transition_time": "2026-10-15T08:01:10Z" },
      { "type": "BackendsHealthy", "status": true, "reason": "AllHealthy", "last_transition_time": "2026-10-15T08:01:30Z" }
    ],
    "config_hash": "9f1c…",
    "cert_id": 31,
    "cert_not_after": "2027-01-13T08:01:00Z",
    "last_applied_at": "2026-10-15T08:01:12Z",
    "servers": [
      { "backend": "app", "address": "127.0.0.1:24001", "down": false, "healthy": true, "checked": true },
      { "backend": "app", "address": "127.0.0.1:24002", "down": true, "healthy": false, "checked": true }
    ]
  },
  "created_at": "2026-10-15T07:59:58Z",
  "updated_at": "2026-10-15T08:01:30Z"
}
```

为空的可选字段会被省略：没有 `message` 的 condition，以及 `tls.mode` 为 `none` 或尚未提供证书时的 `cert_id` / `cert_not_after`。
在 `status.servers` 中，`healthy` 是最近一次探测的结果，只有 `checked` 为 true 时才有意义；在可用性检查器得到该 server 的结果之前（例如刚添加时，或检查器关闭期间），`checked` 为 false。

### Spec 规则

| 字段 | 规则 |
|---|---|
| `name` | 1–40 个字符，`[a-z0-9-]`，以字母或数字开头和结尾；在所有 owner 之间唯一 |
| `owner` | 1–64 个字符，`[A-Za-z0-9._-]` |
| `hosts` | 至少一个；DNS 名称（允许 IDN，按原样存储并渲染为 punycode）；`*.` 通配符仅在 `tls.mode` 为 `cert` 或 `none`，或使用 `dns01` 质询时允许；一个主机只能属于一个 Ingress |
| `backends[].name` | 1–16 个字符，`[a-z0-9-]`；在 Ingress 内唯一 |
| `backends[].lb` | `round_robin`（默认）、`least_conn`、`ip_hash` |
| `backends[].servers[].address` | `host:port`，其中 IP 主机必须位于 `[ingress] AllowedBackends` 范围内（默认 `127.0.0.1/32`、`::1/128`），端口不限；每个 backend 至少一个 server |
| `routes[].path_prefix` | 以 `/` 开头，不含空格、`;`、`{`、`}`、引号或 `#`；在 Ingress 内唯一；最长前缀优先 |
| `routes[].backend` | 必须是该 Ingress 中某个 backend 的名称 |
| timeouts | nginx 时间值（`60s`、`5m`） |
| sizes | nginx 大小值（`0`、`512k`、`64m`、`1g`） |
| `allow_cidrs` | IPv4/IPv6 CIDR 或地址；为空表示允许所有人；路由级列表会替换该路由的 policy 列表 |
| `tls.mode` | `none`、`acme`（签发并续期）、`cert`（使用 `cert_id`） |
| `tls.challenge` | `http01`（默认）或 `dns01`（需要 `dns_credential_id`） |
| `tls.key_type` | 与证书签发相同（`2048`、`3072`、`4096`、`P256`、`P384`）；默认 `2048` |
| `policy.rate_limit` | 按客户端地址限流；`rps` 1–10000，`burst` 0–100000 |
| `policy.extra_headers` | 请求头名称 `[A-Za-z0-9-]`，值不含 `;`、`{`、`}`、引号、换行 |

## 生成的文件

| 文件 | 内容 |
|---|---|
| `sites-available/ingress-<name>` 及 `sites-enabled` 中的链接 | server 块 |
| `conf.d/upstream-ig-<name>-<backend>.conf` | 每个 backend 对应一个受管理的上游，带共享内存区 |

网站记录的 `managed_by` 被设为 `ingress:<name>`，网站 API 会返回该字段。
常规网站 API 会拒绝保存、重命名、启用、禁用、删除此类网站、将其置于维护模式（单个和批量端点均如此）或切换其证书自动续期，并返回 `ErrSiteManagedByIngress`，除非请求强制执行：对 `POST /sites/:name` 在 JSON 请求体中使用 `"force": true`，其他端点使用 `?force=1`。
配置编辑器（保存、覆盖创建、重命名、删除，以及节点接收的同步批次）同样会拒绝生成的网站文件和上游文件。
上游 API 会拒绝更新、删除、切换其中的 server 或转换为生成的上游（upstream scope `40908`，409），除非传入 `?force=1`。MCP 工具从不强制执行。
强制修改会在下一次调和时被覆盖。
这些上游会像其他受管理的上游一样显示在上游页面和可用性检查器中。

以 `ig-` 开头的上游名称为 Ingress 保留：上游 API 会拒绝创建此类上游（upstream scope `40023`）。
如果使用生成名称的受管理上游文件并非由该 Ingress 写入，Ingress 会拒绝覆盖它（`40905`）。

渲染出的网站文件大致如下（`<zone>` 为 `ig_` 加上把 `-` 替换为 `_` 的 Ingress 名称，此处为 `ig_tm_prod_insure_ocr`）：

```nginx
# Managed by Nginx UI ingress tm-prod-insure-ocr
# Generated from the ingress spec; manual changes are overwritten.
# Upstreams: ig-tm-prod-insure-ocr-app
map $http_upgrade $ig_tm_prod_insure_ocr_connection {   # any websocket route
    default upgrade;
    '' close;
}
limit_req_zone $binary_remote_addr zone=ig_tm_prod_insure_ocr:10m rate=20r/s;   # policy.rate_limit
server {
    listen 80;
    listen [::]:80;
    server_name ocr.example.com;
    # ACME HTTP-01 challenge, proxied to Nginx UI
    location ^~ /.well-known/acme-challenge { ... }
    location / { return 301 https://$host$request_uri; }   # tls.redirect_https and cert ready
}
server {                                                    # only when the certificate is ready
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name ocr.example.com;
    ssl_certificate     /etc/nginx/ssl/ocr.example.com_RSA2048/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/ocr.example.com_RSA2048/private.key;
    location / {
        allow 203.0.113.0/24; deny all;
        limit_req zone=ig_tm_prod_insure_ocr burst=40 nodelay;
        client_max_body_size 64m;
        proxy_pass http://ig-tm-prod-insure-ocr-app;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;             # websocket
        proxy_set_header Connection $ig_tm_prod_insure_ocr_connection;   # websocket; "" otherwise
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

## 端点

| 方法 | 路径 | 请求体 | 响应 |
|---|---|---|---|
| GET | `/ingresses?owner=<owner>` | — | `{ "data": [Ingress] }` |
| GET | `/ingresses/:name` | — | `Ingress` |
| PUT | `/ingresses/:name?wait=<duration>&dry_run=1` | `UpsertRequest` | `Ingress`；试运行时为 `ApplyResult` |
| DELETE | `/ingresses/:name?owner=<owner>&force=1&purge_cert=1` | — | `{ "message": "ok" }` |
| PUT | `/ingresses?owner=<owner>&prune=1&wait=<duration>` | `SyncRequest` | `{ "data": [Ingress] }`（同步后该 owner 的 Ingress） |
| PATCH | `/ingresses/:name/backends/:backend/servers?wait=<duration>` | `ServerPatch` | `Ingress` |
| POST | `/ingresses/:name/reconcile?wait=<duration>` | — | `Ingress` |

```jsonc
// UpsertRequest
{ "owner": "telemetry-prod", "external_id": "mac-service:42", "labels": {}, "namespace_id": 0, "force": false, "spec": { /* IngressSpec */ } }

// SyncRequest: every ingress the owner wants; names not listed are deleted when prune=1
{ "items": [ { "name": "tm-prod-insure-ocr", "external_id": "…", "labels": {}, "namespace_id": 0, "spec": { /* IngressSpec */ } } ] }

// ServerPatch
{ "owner": "telemetry-prod", "address": "127.0.0.1:24002", "down": true }

// ApplyResult (dry run)
{ "name": "…", "changed": true, "rendered": { "site_name": "…", "site_content": "…", "upstreams": { "ig-…-app": "…" }, "hash": "…" }, "test_output": "", "error": "" }
```

语义：

- **PUT 是幂等的。** 仅当 spec 发生变化时，版本（generation）才会递增；无论是否变化，Ingress 都会被加入应用队列，因此重复发送 PUT 可以重试失败的应用。
- **所有者（owner）。** 创建时设置所有者。使用不同的所有者写入 Ingress 会失败并返回 `ErrOwnerMismatch`（409），除非设置了 `force`，此时会转移所有权。带 `prune=1` 的同步只会删除指定所有者的 Ingress。
- **应用是异步的。** 写入在 spec 存储后即返回。约一秒内到达的变更会合并应用，只执行一次 `nginx -t` 和一次重载。指定 `wait`（最长 60 秒）时，请求会阻塞，直到本请求加入队列的那次应用完成且 `status.observed_generation` 达到新版本，或等待超时（`ErrWaitTimeout`，408；变更仍保留在队列中）。由于等待的是本请求自己的应用，spec 未变的重试会返回这次重试的结果，而不是之前的失败。带 `prune=1` 和 `wait` 的同步还会等待被清理的 Ingress 删除完成（否则返回 `ErrDeleteWaitTimeout`，408）。未指定 `wait` 时请轮询 `GET /ingresses/:name`。
- **失败的应用是原子的。** 该次应用写入的所有文件都会被恢复，nginx 继续运行之前的配置，出错的 Ingress 会得到 `ConfigApplied=false`，测试输出写在 message 中。
- **摘除（Draining）。** 对 `PATCH …/servers` 传入 `down: true` 会为该 server 设置 `down`；重载后 nginx 不再向其发送新请求，处理中的请求会正常完成。设置 `down: false` 可将其恢复。
- **删除**会移除已启用的链接、网站文件和上游文件，重载 nginx，并删除记录。证书记录和文件会保留，这样重新创建 Ingress 时不会触发 ACME 速率限制，除非指定 `purge_cert=1`；保留的记录会停止自动续期，直到再次创建同名的 ACME Ingress。`purge_cert=1` 还会删除空的证书目录。
- **写入优先于进行中的删除。** 删除与发起时的版本绑定。如果在删除移除记录之前有 PUT、PATCH 或同步写入了该 Ingress，Ingress 会被保留（其文件会重新应用），删除则失败并返回 `ErrDeleteConflict`（409）。

## 状态

`phase` 汇总各 condition 的状态：

| 阶段 | 条件 |
|---|---|
| `Error` | 最近一次应用失败；之前的配置仍在生效。优先于 `Pending`：等待应用的新版本不会掩盖这次失败 |
| `Pending` | `observed_generation < generation`，且 worker 尚未处理 |
| `Applying` | worker 正在应用 |
| `Ready` | `ConfigApplied` 为 true，`CertReady` 为 true 且 reason 为 `Issued`、`Existing` 或 `NotRequired`，`BackendsHealthy` 为 true 或 `Unknown` |
| `Degraded` | 配置已应用，但证书尚未就绪、正在提供的证书尚未覆盖 spec（`CertReady` 为 true 但 reason 为其他值），或某个 backend 没有未被标记为 down 且健康的 server |

Condition 的 reason：

| Condition | Reasons |
|---|---|
| `ConfigApplied` | `Applied`、`TestFailed`、`ReloadFailed`、`RenderFailed`、`Drifted`（被调和重写） |
| `CertReady` | `Issued`、`Existing`、`NotRequired`、`Issuing`、`DNSNotReady`、`IssueFailed`、`CertNotFound`、`Expired` |
| `BackendsHealthy` | `AllHealthy`、`PartiallyHealthy`、`NoHealthyServer`、`Unknown` |

当之前的证书仍为尚未被完全覆盖的 spec（新增了主机或修改了 `tls.key_type`）维持 HTTPS 监听时，`CertReady` 为 true，reason 为 `Issuing`、`DNSNotReady`、`IssueFailed` 或 `CertNotFound`。
对 ACME Ingress 而言，`CertNotFound` 表示自 Nginx UI 启动以来尚未进行签发；下一次应用或调和会开始签发。

### 证书

- `acme`：在证书存在之前，只渲染 HTTP 监听和质询 location。Nginx UI 会先检查主机：每个主机都必须有 A 或 AAAA 记录，且没有排除该 CA 的 CAA 记录；只有当本服务器的某个网络接口上有公网地址时，主机解析到不属于本服务器的地址才会被视为问题（在 NAT 之后、容器中或 CDN 之后，这种不一致是正常的）。随后检查 nginx 是否把每个主机的质询路径转发给 Nginx UI。检查失败时为 `DNSNotReady`（路由检查失败时为 `IssueFailed`），并以 1 到 30 分钟的退避间隔重试。之后使用默认或指定的 ACME 用户签发，并启用自动续期。签发成功后才会渲染 HTTPS 监听。续期由常规的自动证书任务完成。证书文件位于 `ssl/<第一个主机>_<密钥类型>/`，密钥类型为规范化后的名称（`RSA2048`、`EC256` 等）。签发状态只保存在内存中：重启后，worker 和调和会为证书缺失或不完整的每个 ACME Ingress 重新开始签发。新增主机或修改 `tls.key_type` 时，在新证书签发之前会继续提供当前证书（前提是它仍然有效，且在修改密钥类型时覆盖所有主机）；新证书签发后，被取代的其他密钥类型证书会停止自动续期。
- `cert`：直接使用证书 `cert_id`；它必须覆盖所有主机。
- `none`：仅 HTTP。

无论间隔如何设置，第一次调和都会在 Nginx UI 启动一分钟后运行，之后每隔 `[ingress] ReconcileIntervalMinutes`（默认 5）分钟运行一次。
它会重写内容与 spec 不再一致的生成文件，刷新 `cert_not_after` 和 server 健康状态，并恢复证书签发。
应用成功后也会立即刷新所应用 Ingress 的健康状态，因此新的 server 无需等待下一次调和。

## 在界面中查看 Ingress

侧边菜单中的 **Ingress** 会列出所有 Ingress，包括所有者、主机、阶段、证书到期时间、后端健康状况（健康 server 数量与总数，以及用 `down` 摘除的数量；在检查器得到结果之前显示为灰色，未检查的 server 不会被算作不健康），以及已应用版本与最新版本的对比（`gen 2/3` 表示版本 3 仍在等待应用）。
可以按所有者筛选，或按名称、主机、外部 ID 搜索。页面打开期间，列表每隔几秒自动刷新。

详情抽屉以可读形式展示 spec（主机、带有各 server 健康状况的 backend（未检查时显示 **无数据**）、路由、TLS 和 policy）、各 condition 及其 reason 和 message（应用失败时包含 `nginx -t` 的输出），以及生成的网站配置。可执行两个操作：

- **立即调和** 会将该 Ingress 加入应用队列，文件发生漂移时会被重写（`POST /ingresses/:name/reconcile`）。
- **删除** 会使用 `force=1` 和其所有者删除该 Ingress，也可选择同时删除其证书。拥有它的控制器通常会在下一次同步时重新创建它，因此要彻底删除，请在控制器中移除它。

除此之外，Ingress 在界面中是只读的：请通过拥有它们的控制器进行修改。

生成的网站在网站列表中带有 **Ingress** 标签，并在网站编辑器中显示指回该 Ingress 的警告。此类网站的保存、重命名、启用、禁用、置于维护模式和删除操作均被锁定。在编辑器中打开 **仍然编辑**（或勾选网站列表批量工具栏中的复选框），即可携带强制标志发送修改；下一次调和仍会覆盖该修改。

## 设置

```ini
[ingress]
AllowedBackends = 127.0.0.1/32
AllowedBackends = ::1/128
ReconcileIntervalMinutes = 5
```

`AllowedBackends` 列出 backend server 可以指向的 CIDR 或地址，这些地址上的任意端口都允许。
它是受保护的设置：只能在 `app.ini` 或环境变量（`NGINX_UI_INGRESS_ALLOWED_BACKENDS`）中修改，不能通过 API 修改。

## 错误

错误使用常规响应体 `{ "scope": "ingress", "code": 40901, "message": "…", "params": [...] }`。
与大多数 Nginx UI 端点不同，Ingress 端点返回的 HTTP 状态码由错误码的前三位数字决定，因此控制器仅凭状态码即可判断。如需针对具体错误处理，请匹配 `scope` 和 `code`。

Scope `ingress`：

| Code | HTTP | 含义 |
|---|---|---|
| 40000 | 400 | 请求体或查询参数无效 |
| 40001 | 400 | Ingress 名称无效 |
| 40002 | 400 | owner 无效 |
| 40003 | 400 | host 无效 |
| 40004 | 400 | backend 无效 |
| 40005 | 400 | backend server 超出允许的地址或端口范围 |
| 40006 | 400 | route 无效 |
| 40007 | 400 | tls 无效 |
| 40008 | 400 | policy 无效 |
| 40009 | 400 | backend 中未找到该 server |
| 40401 | 404 | 未找到 Ingress |
| 40801 | 408 | 等待超时（变更仍在队列中） |
| 40802 | 408 | 等待被清理的 Ingress 删除超时（删除仍在队列中） |
| 40901 | 409 | 由其他所有者持有 |
| 40902 | 409 | 该 host 已由其他 Ingress 提供服务 |
| 40903 | 409 | 已存在与生成名称相同且不受此 Ingress 管理的网站 |
| 40904 | 409 | Ingress 在删除过程中被写入，已被保留 |
| 40905 | 409 | 已存在使用生成名称且不受此 Ingress 管理的受管理上游 |
| 50001 | 500 | 应用失败 |
| 50002 | 500 | 证书未就绪 |

Scope `site`：`40901`（409）该网站由 Ingress 管理。

Scope `upstream`：`40908`（409）该上游由 Ingress 管理；`40023` 以 `ig-` 开头的上游名称为 Ingress 保留。
