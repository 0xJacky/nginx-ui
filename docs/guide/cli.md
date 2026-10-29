# Command Line Interface

The `nginx-ui ctl` command manages a running Nginx UI instance through its
management API. It is intended for infrastructure-as-code, configuration-as-code,
deployment automation, and remote administration.

## Create an access token

1. Sign in to Nginx UI as an administrator.
2. Open **Preferences > Access Tokens**.
3. Create a token with the smallest required scope and an expiration date.
4. Copy the token immediately. Nginx UI does not display it again.

The API scopes are:

| Scope | Access |
| --- | --- |
| `api:read` | `GET`, `HEAD`, and `OPTIONS` management API requests, with stored secrets hidden |
| `api:write` | Mutating management API requests and API read access, including stored secrets |

`api:read` is a read-only role and never receives stored secrets. Responses
keep their shape and every non-secret field, but secret values are replaced by
`__NGINX_UI_REDACTED__`. This covers notifier configuration values, site health
check header values, DNS provider credentials attached to DNS domains, and the
private key in certificate details. The DNS credential, ACME account and S3
auto-backup endpoints omit their credential fields for every service token. Reading a
private key file through `GET /api/config`, either a `.key` file or any file
containing a PEM private key block, is refused with `403` and error code
`40019`, and private key revisions in configuration history are redacted.
Certificates and other configuration files stay readable. The site navigation
WebSocket feed of a remote node (`X-Node-ID`) is refused with `403` and error
code `40301`, because the controller cannot redact relayed frames. Use
`GET /api/site_navigation` instead. Use an `api:write` token when automation
needs to export keys or read stored credentials.

MCP scopes are independent of API scopes. Granting `mcp:write` does not grant
management API access, and granting `api:write` does not grant MCP access.

Service tokens cannot access interactive account security operations, create or
modify interactive users, reveal protected settings, manage other service
tokens, or open the web terminal. The `api:read` scope may list and inspect
users, but user creation, modification, deletion, and recovery require an
authenticated interactive administrator session.

## Configure the client

Set the endpoint and keep the token in a file readable only by the account that
runs the automation:

```bash
export NGINX_UI_CTL_ENDPOINT=https://nginx-ui.example.com
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users list
```

The endpoint can also be supplied with `--endpoint` or
`NGINX_UI_CTL_ENDPOINT`. The token can be supplied through
`NGINX_UI_CTL_TOKEN`, `--token-file`, or `--token-stdin`. Prefer a secret file
or standard input so the token does not appear in command history or process
arguments.

For a private certificate authority, pass its PEM bundle with `--ca-file`.
Use `--node-id` to route a supported request to a specific cluster node.

## Common operations

List users with an `api:read` service token:

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token users list
```

Create a user with an interactive administrator token:

```bash
nginx-ui ctl --token-file /run/secrets/admin-session-token users create \
  --name deploy-user --password-file /run/secrets/deploy-user-password
```

The predefined-user environment variables remain useful for the initial user
in skip-installation deployments. After installation, use the web interface or
`ctl users create` with an interactive administrator token. Administrators who
have two-factor authentication enabled should use the web interface so the
secure-session verification can be completed.

List certificates and register certificate files that already exist on the
Nginx UI server:

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token certificates list
nginx-ui ctl --token-file /run/secrets/nginx-ui-token certificates import \
  --name example.com \
  --cert /etc/nginx/ssl/example.com/fullchain.pem \
  --key /etc/nginx/ssl/example.com/privkey.pem
```

The dedicated certificate commands omit certificate and private-key PEM data
from their output so CI logs do not capture key material.

Inspect and control Nginx:

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx status
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx test
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx reload
nginx-ui ctl --token-file /run/secrets/nginx-ui-token nginx restart
```

## Call any management API

The generic `api` command provides coverage for management operations that do
not have a dedicated command:

```bash
nginx-ui ctl --token-file /run/secrets/nginx-ui-token api sites?page=1
nginx-ui ctl --token-file /run/secrets/nginx-ui-token api \
  --method POST --data-file site.json sites
```

Paths are resolved below `/api`. Query strings are preserved, but absolute URLs
are rejected so credentials cannot be redirected to another host. Request and
response bodies are limited to 16 MiB.

## Token lifecycle

Creating, rotating, and revoking service tokens requires an interactive
administrator token, including any required two-factor verification:

```bash
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens list
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens create \
  --name ci --scope api:write --expires-at 2027-01-01T00:00:00Z
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens rotate TOKEN_ID
nginx-ui ctl --token-file /run/secrets/admin-session-token tokens revoke TOKEN_ID
```

Rotation invalidates the previous token immediately. Revocation is permanent.
