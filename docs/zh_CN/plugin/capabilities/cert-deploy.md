---
outline: [2, 3]
---

# 证书部署

`cert.deploy` 插件把 Nginx UI 签发或续期的证书推送到在 Nginx UI 之外终结 TLS 的地方：CDN、云负载均衡、邮件服务器、NAS。它声明目标类型；用户配置目标并把它们绑定到证书，每当绑定的证书签发或续期，以及用户手动触发时，Nginx UI 都会调用插件，并传入证书、私钥和证书链。

| 方法 | 必需 | 用途 |
| --- | --- | --- |
| `deploy.push` | 是 | 把一个证书推送到一个目标，或检查能否推送。 |
| `deploy.validate` | 否 | 在不访问目标的情况下检查目标配置。 |

插件会收到私钥，因此必须请求 `cert.deploy` 权限，它会如实告诉批准插件的人这一点。插件还应请求 `network`。

## 声明目标类型 {#declaring-target-kinds}

```json [plugin.json]
"capabilities": ["cert.deploy"],
"permissions": ["cert.deploy", "network"],
"deploy": {
  "targets": [
    {
      "code": "mycdn",
      "name": "MyCDN",
      "configuration": {
        "fields": [
          { "key": "api_token", "display_name": "API token", "required": true, "secret": true },
          { "key": "zone_id", "display_name": "Zone ID", "help_text": "Zone the certificate is bound to", "required": true }
        ]
      }
    }
  ]
}
```

`code`、`name` 和 `configuration.fields` 的用法与[通知渠道](./notify.md#configuration-form)相同。`code` 以 `kind` 的形式传入。

## 推送 {#pushing}

```json
{
  "jsonrpc": "2.0", "id": 47, "method": "deploy.push",
  "params": {
    "kind": "mycdn",
    "config": { "api_token": "tok_live_xxx", "zone_id": "zone_123" },
    "certificate": {
      "name": "example.com",
      "domains": ["example.com", "www.example.com"],
      "certificate_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\n...\n-----END EC PRIVATE KEY-----\n",
      "chain_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "not_after": "2026-12-20T08:15:00Z"
    },
    "dry_run": false
  }
}
```

| 字段 | 含义 |
| --- | --- |
| `kind`、`config` | 目标类型及其表单的值。 |
| `certificate.name` | 证书在 Nginx UI 中的名称。 |
| `certificate.domains` | 证书覆盖的域名和 IP 地址。 |
| `certificate.certificate_pem` | 叶证书。 |
| `certificate.private_key_pem` | 叶证书的私钥（PKCS #1、SEC 1 或 PKCS #8）。 |
| `certificate.chain_pem` | 中间证书，叶证书的签发者在前。没有时为空。 |
| `certificate.not_after` | 叶证书的过期时间，RFC 3339。 |
| `dry_run` | 只检查，参见下文。 |

目标需要完整证书链时，把 `certificate_pem` 和 `chain_pem` 拼接起来。

目标接受证书后回复 `{ "message": "..." }`，附上一段变更摘要，例如创建或替换了哪个资源。推送必须是幂等的：推送目标已经在使用的证书也要成功，因为 Nginx UI 会重试，用户也可能再次推送。原因在于配置（令牌已吊销、区域不存在）时回复带 `data.field` 的 `-32003`，其他情况回复 `-32000`。

### 试运行 {#dry-run}

`dry_run: true` 时，不要修改目标上的任何内容。用只读调用检查真正推送所需的条件（目标可访问、凭据有效、资源存在），对必填字段为空的配置回复 `-32003`，并在 `message` 中说明真正推送时会做什么。

### 保护私钥 {#protecting-the-key}

::: danger 警告
- 私钥和 `config` 中的每个值都是机密：绝不能出现在日志、错误或 `message` 中。
- 不要把私钥写入磁盘，除非目标本身需要文件（通过 SSH 推送的插件把它写到目标上，而不是自己的数据目录中）。
- 回复之后不要保留私钥，并且只推送到本次调用的 `config` 指定的地方。
:::

## 校验 {#validating}

`deploy.validate` 包含 `kind` 和 `config`。在不访问目标的情况下检查配置：对第一个问题回复带 `data.field` 的 `-32003`，否则回复 `{}`。Nginx UI 在保存目标前调用它，并且不会保存被它拒绝的目标。

## Nginx UI 如何使用它 {#how-nginx-ui-uses-it}

- 只有在插件已启用并持有 `cert.deploy` 权限时才提供它的目标类型。
- 目标可以绑定到一个证书或所有证书。绑定的证书签发或续期时，Nginx UI 把它推送到每个已启用的目标，推送失败时在 30 秒、2 分钟和 10 分钟后重试。每次尝试都会重新读取证书。
- 用户可以手动推送（尝试一次，不重试），也可以用试运行测试目标。
- 每次推送都会连同消息或错误以及尝试次数一起记录，并显示在证书旁边。

`deploy.push` 最多等待 5 分钟，`deploy.validate` 最多等待 30 秒。
