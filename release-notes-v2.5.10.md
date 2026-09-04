## Features

- Add Quick Setup to site creation and editing with validated redirect and reverse-proxy configuration templates.
- Show live progress while upgrading child-node authentication.
- Add an option to disable authoritative DNS propagation checks during certificate issuance.

## Bug Fixes

- Keep long-running certificate issuance WebSocket connections alive and report when the connection closes before completion.
- Preserve the configured DNS propagation delay before ACME validation when active propagation checks are disabled.
- Honor separate Cloudflare zone and DNS API tokens when resolving zones and updating records.
- Automatically migrate legacy certificate paths and expose migration results in certificate management.
- Complete TLS-aware upstream health checks without sending application requests to HTTPS backends.
- Allow site TLS certificates and keys defined through included NGINX configuration files.
- Clarify the DNS zone and DDNS record workflow and display fully qualified record names.
- Show the underlying error when a remote node is unreachable.
- Build cluster WebSocket URLs without rewriting unrelated `http` text.
- Emit forwarded maps independently of the WebSocket toggle in Quick Setup.
- Avoid splitting multi-byte UTF-8 characters when truncating LLM-generated session titles.
- Stop upstream comments from being repeated on following directives.
- Correct NGINX log-level constants so error entries are classified correctly.
- Prevent a panic when an S3 backup path points to the bucket root.
- Keep `dateext` logrotate files grouped with their source log.
- Point Debian `nginx.conf` documentation links at the current branch.

## Contributors

@0xJacky
@VXNCXNX
@ugurcsen
