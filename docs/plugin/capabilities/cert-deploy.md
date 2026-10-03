---
outline: [2, 3]
---

# Certificate Deployment

A `cert.deploy` plugin pushes certificates Nginx UI issued or renewed to
places that terminate TLS outside Nginx UI: a CDN, a cloud load balancer, a
mail server, a NAS. It declares kinds of target; a person configures targets
and binds them to certificates, and Nginx UI calls the plugin with the
certificate, its private key and its chain whenever a bound certificate is
issued or renewed, and whenever the person asks.

| Method | Required | Purpose |
| --- | --- | --- |
| `deploy.push` | yes | Push one certificate to one target, or check that it could be pushed. |
| `deploy.validate` | no | Check a target configuration without contacting it. |

The plugin receives private keys, so it must request the `cert.deploy`
permission, which tells the person approving it exactly that. It should also
request `network`.

## Declaring Target Kinds

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

`code`, `name` and `configuration.fields` work as for
[notification channels](./notify.md#configuration-form). The `code` arrives as
`kind`.

## Pushing

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

| Field | Meaning |
| --- | --- |
| `kind`, `config` | The target kind and the values of its form. |
| `certificate.name` | Name of the certificate in Nginx UI. |
| `certificate.domains` | The domains and IP addresses it covers. |
| `certificate.certificate_pem` | The leaf certificate. |
| `certificate.private_key_pem` | The private key of the leaf (PKCS #1, SEC 1 or PKCS #8). |
| `certificate.chain_pem` | The intermediate certificates, issuer of the leaf first. Empty when there is none. |
| `certificate.not_after` | Expiry of the leaf, RFC 3339. |
| `dry_run` | Check only, see below. |

Concatenate `certificate_pem` and `chain_pem` when the target wants the full
chain in one piece.

Answer `{ "message": "..." }` once the target accepted the certificate, with a
short summary of what changed, such as the resource created or replaced. A
push must be idempotent: pushing a certificate the target already serves
succeeds, since Nginx UI retries and people push again. Errors are `-32003`
with `data.field` for causes in the configuration (a revoked token, a zone
that does not exist) and `-32000` otherwise.

### Dry Run

With `dry_run: true`, change nothing at the target. Check what a real push
needs with read-only calls (the target answers, the credentials work, the
resource exists), reject a configuration with an empty required field with
`-32003`, and answer with a `message` saying what a real push would do.

### Protecting the Key

::: danger
- The private key and every `config` value are secrets: never in a log, an
  error or the `message`.
- Do not write the key to disk, except where the target itself needs a file
  (a plugin pushing over SSH writes it on the target, not in its data
  directory).
- Do not keep the key after answering, and push only to what the call's
  `config` names.
:::

## Validating

`deploy.validate` has `kind` and `config`. Check the configuration without
contacting the target: `-32003` with `data.field` for the first problem, `{}`
otherwise. Nginx UI calls it before saving a target and does not save one it
rejects.

## How Nginx UI Uses It

- Target kinds are offered only while the plugin is enabled and holds the
  `cert.deploy` permission.
- A target is bound to one certificate or to all of them. When a bound
  certificate is issued or renewed, Nginx UI pushes it to every enabled target
  and retries a failed push after 30 seconds, 2 minutes and 10 minutes.
  Nginx UI reads the certificate afresh for every attempt.
- A person can push on demand (one attempt, no retries) and test a target
  with a dry run.
- Every push is recorded with its message or error and the number of attempts,
  and shown next to the certificate.

Nginx UI waits 5 minutes for `deploy.push` and 30 seconds for
`deploy.validate`.
