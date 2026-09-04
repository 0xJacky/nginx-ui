## Features

- Run the public demo on Cloudflare Containers, with a browser-simulated terminal and fabricated state so every screen can be explored without configuring a provider.
- Cover the demo with an end-to-end Playwright suite, including the site navigation socket and classified site health reasons.

## Bug Fixes

- Test the configuration with `nginx -t` before keeping a saved file, and restore the previous content when the test fails. An invalid config can no longer be left on disk to break the next nginx start. Covers the config editor, config creation, cluster sync, stream saves, and the MCP add tool (#1797).
- Write renewed certificates back to the paths nginx actually loads, and stop the post-renewal database write from failing silently. Renewals no longer land in a new directory while the vhost keeps serving the expiring certificate (#1794).
- Accept the WebSocket token from the query string on `/api/site_navigation_ws` and `/api/code_completion`, fixing the immediate 403 that broke the site navigation sidebar (#1793).
- Bound index rebuilds with a global task limit and derive CPU and memory budgets from cgroup limits instead of host specifications, preventing memory and CPU exhaustion after upgrading inside a container (#1792).
- Stop site health probes from failing on self-signed certificates, and show a classified reason on the site card. Probes that carry custom headers or a request body still validate the certificate chain (#1790).
- Make the segmented one-time-code inputs discoverable by password managers by wrapping them in a real form with stable names and `autocomplete="one-time-code"` (#1786).
- Remove the enabled symlink when `nginx -t` or the reload rejects a stream or a configuration that was just enabled, so a broken configuration is no longer left active for the next nginx start.
- Write ACME private keys with owner-only permissions, and tighten keys that earlier versions created world-readable.
- Let nodes in the recycle bin be recovered or permanently deleted (#1796).
- Keep account and secure-session routes on the controller when a remote node is selected, so administrator actions work again through the cluster switcher (#1788).
- Skip the searcher hot swap until the first indexing task creates a shard, removing the `no healthy shards available` errors when advanced indexing is first enabled (#1787).
- Use the node host for proxied cluster requests, and diagnose proxy rejections without echoing the upstream response body back to the caller.
- Require an interactive administrator to create, modify, or test an external notifier, and localize certificate expiry alerts.
- Stop the analytics stream spinning on errors and leaking goroutines after a client disconnects, expire idle HTTP connections, and coalesce site sweeps triggered by configuration scans.
- Encode filesystem paths correctly in log entry queries instead of sending them raw in the query string.
- Prevent network spikes on the dashboard after a reconnect, and limit the initial range of the structured log view.

## Contributors

@0xJacky
