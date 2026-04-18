# Site Check

The Site Checker probes every `server_name` Nginx serves to keep the Dashboard
status indicators current. This section controls how aggressively it runs.

If your `server_name` entries resolve to ingress services that return many A
records (ngrok, AWS load balancers, Cloudflare), the historical defaults could
open enough concurrent outbound TCP flows to exhaust conntrack tables on
consumer routers (UniFi etc.). See
[issue #1608](https://github.com/0xJacky/nginx-ui/issues/1608).

## Enabled

- Type: `bool`
- Default: `true`
- Version: `>= v2.3.6`

When `false`, the Site Checker service does not start: no periodic sweeps run
and no outbound connections are opened on its behalf. The Dashboard will keep
showing the last known state (or empty state on first start). Disable this when
you do not need automated health checks, or when the checker is causing
upstream / network problems.

## Concurrency

- Type: `int`
- Default: `5`
- Range: `[1, 20]`
- Version: `>= v2.3.6`

The maximum number of concurrent health checks during a single sweep. Lower
values reduce burstiness; higher values complete a full sweep faster. The
checker also bounds connections per host (`MaxConnsPerHost = 2`), so even
hostnames with many A records will not open more than two concurrent flows
each.

## IntervalSeconds

- Type: `int`
- Default: `300`
- Minimum: `30`
- Version: `>= v2.3.6`

How often, in seconds, the Site Checker re-sweeps every collected site. The
default of 5 minutes balances freshness against load. Values below 30 are
clamped back to the default.

## Example

```ini
[site_check]
Enabled         = true
Concurrency     = 5
IntervalSeconds = 300
```
