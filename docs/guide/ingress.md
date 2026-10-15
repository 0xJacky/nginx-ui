# Ingress API

An **ingress** is a reverse proxy entry that an external controller declares
and Nginx UI keeps in place. The controller sends the desired state; Nginx UI
renders the nginx config, issues the certificate, tests and reloads nginx, and
reports the result. Sites generated this way are marked as managed and should
not be edited by hand: the next reconcile rewrites them.

Typical use: a control plane exposes services that reach this host through a
tunnel (for example frp remote ports bound on `127.0.0.1`) under public
host names on port 443.

## Authentication

Every endpoint is under `/api` and accepts the usual credentials:

- signed node requests (`Signature` / `Signature-Input`) or the legacy
  `X-Node-Secret` header — the recommended choice for controllers;
- a service token with the `api:read` / `api:write` scope.

Write endpoints require a secure session; node requests and `api:write`
tokens satisfy it. The `owner` field is a guard against two controllers
overwriting each other, not an authorization boundary.

## Resource

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
      { "type": "ConfigApplied", "status": true, "reason": "Applied", "message": "", "last_transition_time": "2026-10-15T08:00:00Z" },
      { "type": "CertReady", "status": true, "reason": "Issued", "message": "", "last_transition_time": "2026-10-15T08:01:10Z" },
      { "type": "BackendsHealthy", "status": true, "reason": "AllHealthy", "message": "", "last_transition_time": "2026-10-15T08:01:30Z" }
    ],
    "config_hash": "9f1c…",
    "cert_id": 31,
    "cert_not_after": "2027-01-13T08:01:00Z",
    "last_applied_at": "2026-10-15T08:01:12Z",
    "servers": [
      { "backend": "app", "address": "127.0.0.1:24001", "down": false, "healthy": true },
      { "backend": "app", "address": "127.0.0.1:24002", "down": true, "healthy": false }
    ]
  },
  "created_at": "2026-10-15T07:59:58Z",
  "updated_at": "2026-10-15T08:01:30Z"
}
```

### Spec rules

| Field | Rule |
|---|---|
| `name` | 1–40 chars, `[a-z0-9-]`, starts and ends with a letter or digit; unique across owners |
| `owner` | 1–64 chars, `[A-Za-z0-9._-]` |
| `hosts` | at least one; DNS names (IDN allowed, stored as given and rendered as punycode); `*.` wildcards only with `tls.mode` `cert` or `none` or a `dns01` challenge; a host may belong to one ingress only |
| `backends[].name` | 1–16 chars, `[a-z0-9-]`; unique in the ingress |
| `backends[].lb` | `round_robin` (default), `least_conn`, `ip_hash` |
| `backends[].servers[].address` | `host:port` with an IP host inside `[ingress] AllowedBackends` (default `127.0.0.1/32`, `::1/128`); at least one server per backend |
| `routes[].path_prefix` | starts with `/`, no spaces, `;`, `{`, `}`, quotes or `#`; unique in the ingress; longest prefix wins |
| `routes[].backend` | names a backend of the ingress |
| timeouts | nginx time values (`60s`, `5m`) |
| sizes | nginx sizes (`0`, `512k`, `64m`, `1g`) |
| `allow_cidrs` | IPv4/IPv6 CIDRs or addresses; empty means everyone; route-level lists replace the policy list for that route |
| `tls.mode` | `none`, `acme` (issue and renew), `cert` (use `cert_id`) |
| `tls.challenge` | `http01` (default) or `dns01` (needs `dns_credential_id`) |
| `tls.key_type` | as in certificate issuance (`2048`, `3072`, `4096`, `P256`, `P384`); default `2048` |
| `policy.rate_limit` | per client address; `rps` 1–10000, `burst` 0–100000 |
| `policy.extra_headers` | header names `[A-Za-z0-9-]`, values without `;`, `{`, `}`, quotes, newlines |

## Generated files

| File | Content |
|---|---|
| `sites-available/ingress-<name>` and the `sites-enabled` link | the server blocks |
| `conf.d/upstream-ig-<name>-<backend>.conf` | one managed upstream per backend, with a shared memory zone |

The site row has `managed_by` set to `ingress:<name>` and the site API returns
it. The regular site API refuses to save, rename, enable, disable, delete or
put in maintenance such a site (single and batch endpoints) with
`ErrSiteManagedByIngress` unless the request forces it: `"force": true` in the
JSON body of `POST /sites/:name`, `?force=1` on the other endpoints. A forced
change is overwritten by the next reconcile. The upstreams show up on the upstream page and in the
availability checker like any other managed upstream.

Rendered server blocks, roughly:

```nginx
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
    ssl_certificate     /etc/nginx/ssl/ocr.example.com_2048/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/ocr.example.com_2048/private.key;
    location / {
        allow 203.0.113.0/24; deny all;
        limit_req zone=ig_tm-prod-insure-ocr burst=40 nodelay;
        client_max_body_size 64m;
        proxy_pass http://ig-tm-prod-insure-ocr-app;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;             # websocket
        proxy_set_header Connection $connection_upgrade;    # websocket
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

## Endpoints

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/ingresses?owner=<owner>` | — | `{ "data": [Ingress] }` |
| GET | `/ingresses/:name` | — | `Ingress` |
| PUT | `/ingresses/:name?wait=<duration>&dry_run=1` | `UpsertRequest` | `Ingress`, or `ApplyResult` for a dry run |
| DELETE | `/ingresses/:name?owner=<owner>&force=1&purge_cert=1` | — | `{ "message": "ok" }` |
| PUT | `/ingresses?owner=<owner>&prune=1&wait=<duration>` | `SyncRequest` | `{ "data": [Ingress] }` (the owner's ingresses after the sync) |
| PATCH | `/ingresses/:name/backends/:backend/servers?wait=<duration>` | `ServerPatch` | `Ingress` |
| POST | `/ingresses/:name/reconcile` | — | `Ingress` |

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

Semantics:

- **PUT is idempotent.** The generation increases only when the spec changes;
  the ingress is queued for apply either way, so repeating a PUT retries a
  failed apply.
- **Owner.** Creating sets the owner. Writing an ingress with a different
  owner fails with `ErrOwnerMismatch` (409) unless `force` is set, which
  transfers it. A sync with `prune=1` only ever deletes the given owner's
  ingresses.
- **Apply is asynchronous.** Writes return once the spec is stored. Changes
  that arrive within about a second are applied together with one
  `nginx -t` and one reload. With `wait`, the request blocks until
  `status.observed_generation` reaches the new generation or the wait
  expires (`ErrWaitTimeout`, 408; the change stays queued). Poll
  `GET /ingresses/:name` otherwise.
- **A failed apply is atomic.** All files written in that apply are restored,
  nginx keeps running the previous config, and the failing ingresses get
  `ConfigApplied=false` with the test output in the message.
- **Draining.** `PATCH …/servers` with `down: true` sets `down` on that server;
  nginx stops sending new requests to it after the reload while requests in
  flight complete. Set `down: false` to put it back.
- **Delete** removes the enabled link, the site file and the upstream files,
  reloads, and deletes the record. The certificate record and files are kept
  so re-creating the ingress does not hit ACME rate limits, unless
  `purge_cert=1`.

## Status

`phase` summarizes the conditions:

| Phase | When |
|---|---|
| `Pending` | `observed_generation < generation` and the worker has not picked it up |
| `Applying` | the worker is applying it |
| `Ready` | `ConfigApplied`, `CertReady` (or TLS mode `none`) and `BackendsHealthy` are all true |
| `Degraded` | the config is applied but the certificate is not ready yet or a backend has no healthy server that is not down |
| `Error` | the last apply failed; the previous config is live |

Condition reasons:

| Condition | Reasons |
|---|---|
| `ConfigApplied` | `Applied`, `TestFailed`, `ReloadFailed`, `RenderFailed`, `Drifted` (rewritten by reconcile) |
| `CertReady` | `Issued`, `Existing`, `NotRequired`, `Issuing`, `DNSNotReady`, `IssueFailed`, `CertNotFound`, `Expired` |
| `BackendsHealthy` | `AllHealthy`, `PartiallyHealthy`, `NoHealthyServer`, `Unknown` |

### Certificates

- `acme`: until a certificate exists only the HTTP listener and the
  challenge location are rendered. Nginx UI first checks that every host
  resolves to this server (`DNSNotReady` otherwise, retried with backoff from
  1 to 30 minutes), then issues with the default or given ACME user and
  enables auto renewal. The HTTPS listener is rendered once issuance
  succeeds. Renewal is the regular auto certificate job.
- `cert`: the certificate `cert_id` is used as is; it must cover every host.
- `none`: HTTP only.

A reconcile pass runs every `[ingress] ReconcileIntervalMinutes` (default 5).
It rewrites generated files whose content no longer matches the spec and
refreshes `cert_not_after` and the server health.

## Settings

```ini
[ingress]
AllowedBackends = 127.0.0.1/32
AllowedBackends = ::1/128
ReconcileIntervalMinutes = 5
```

`AllowedBackends` is a protected setting: it can only be changed in `app.ini`
or the environment (`NGINX_UI_INGRESS_ALLOWED_BACKENDS`), not through the API.

## Errors

Errors use the usual body `{ "scope": "ingress", "code": 40901, "message": "…", "params": [...] }`.
Unlike most Nginx UI endpoints, the ingress endpoints answer with the HTTP
status given by the first three digits of the code, so controllers can rely
on the status alone. Match on `scope` and `code` for specific handling.

Scope `ingress`:

| Code | HTTP | Meaning |
|---|---|---|
| 40001 | 400 | invalid ingress name |
| 40002 | 400 | invalid owner |
| 40003 | 400 | invalid host |
| 40004 | 400 | invalid backend |
| 40005 | 400 | backend server outside the allowed ranges |
| 40006 | 400 | invalid route |
| 40007 | 400 | invalid tls |
| 40008 | 400 | invalid policy |
| 40009 | 404 | server not found in backend |
| 40401 | 404 | ingress not found |
| 40801 | 408 | wait timed out (the change is still queued) |
| 40901 | 409 | owned by another owner |
| 40902 | 409 | host already served by another ingress |
| 40903 | 409 | a site with the generated name exists and is not managed by this ingress |
| 50001 | 500 | apply failed |
| 50002 | 500 | certificate not ready |

Scope `site`: `40901` (409) the site is managed by an ingress.
