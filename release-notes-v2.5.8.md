## Features

- Explain why a node is unreachable directly in the node list. A failed connection now carries the underlying error, and the controller recognises a clock skew between itself and the node — the case where TLS rejects the certificate as not yet valid before node authentication ever runs — and answers it with the commands that fix it instead of a generic failure.
- Show MiniMax model parameters and endpoints in the model metadata.

## Bug Fixes

- Index the nginx default access log even when the configuration declares no `access_log` directive. The log preview resolved the path from `nginx.AccessLogPath` (falling back to the `--http-log-path` reported by `nginx -V`), but indexing only ever considered paths scanned from `access_log` directives, so a setup whose directives are commented out — the Homebrew default — could preview its logs while the indexer reported nothing to index (#1787).
- Keep the log paths discovered from the nginx configuration across an indexing restart. Turning advanced indexing off destroyed the manager that owned them and turning it back on built an empty one, so a rebuild started before the next periodic config scan found zero log groups (#1787).
- Enumerate log groups before clearing index metadata during a full rebuild, so a group that is only known from a previous index can still be rebuilt (#1787).
- Reset the task scheduler when indexing services stop, so automatic indexing of unindexed logs resumes after advanced indexing is turned off and on again (#1787).
- Report a rebuild that found no access log group as a warning with diagnostics instead of a misleading `Successfully completed`, and describe both ways a log path can be discovered instead of pointing only at `access_log` directives (#1787).
- Stop the search cache from panicking with `send on closed channel` when advanced indexing is disabled while a shard hot swap is still clearing it.
- Log why a node authentication attempt was rejected, including the credential type, request method and path, and the remote address. A signature or legacy secret failure previously returned an error with nothing recorded to explain it.
- Add the `X-Forwarded-Proto` and `X-Forwarded-Host` headers to a bundled `nginx-ui.conf` that predates them. The self check only rewrote those headers when they already existed, so an installation created before they were introduced stayed unfixed.
- Extract the interface strings introduced in v2.5.7 so they can be translated: the one-time-code fields, the site health check failure reasons, and the certificate record persistence error.

## Contributors

@0xJacky, @octo-patch
