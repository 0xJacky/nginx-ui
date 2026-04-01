# Http

## GithubProxy

- Type: `string`
- Version: `>= v2.0.0-beta.37`
- Suggestion: `https://cloud.nginxui.com/`

For users who may experience difficulties downloading resources from GitHub (such as in mainland China), this option
allows them to set a proxy for github.com to improve accessibility.

## InsecureSkipVerify

- Version：`>= v2.0.0-beta.37`
- Type: `bool`

This option is used to skip the verification of the certificate of servers when Nginx UI sends requests to them.

## WebSocketTrustedOrigins

- Type: `[]string`
- Default: empty
- Example: `http://localhost:5173,https://admin.example.com`

::: tip
Since Nginx UI uses ticket-based WebSocket authentication, this option is **no longer required** for most deployments.
WebSocket security is now enforced by requiring an explicit short token in the URL query parameter, which can only be obtained through a CSRF-protected API endpoint.
This setting is retained as an optional defense-in-depth measure.
:::

This option allows additional trusted browser origins for authenticated WebSocket connections.

Use it when Nginx UI is accessed through a reverse proxy with a different public origin, through multiple management domains, or during local development where the frontend and backend run on different ports.

Keep this list as small as possible. Same-origin WebSocket requests do not need to be added here.

