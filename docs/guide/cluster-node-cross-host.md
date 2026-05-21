# Cluster Node — Cross-Host Setup

When you want to manage nginx on multiple hosts from a single Nginx UI dashboard, the right tool is the **cluster Node** feature, not the host SSH mode.

## When to use what

| Need | host_via_ssh | cluster Node |
|---|---|---|
| Container on host A managing nginx on host A | ✓ | ✓ (overkill) |
| Container on host A managing nginx on host B | ✗ | ✓ |
| One Web UI viewing configs/logs across hosts | — | ✓ |
| Per-host autonomy if peer unreachable | — | ✓ |

## Recommended topology

```
                        ┌──────────────┐
                        │  Your browser │
                        └──────┬───────┘
                               │
                       ┌───────▼────────┐
                       │  Host A         │
                       │  nginx-ui (lead)│
                       │  └─► host_via_ssh ─► host A nginx (optional)
                       └───────┬─────────┘
                               │ cluster federation
                ┌──────────────┼──────────────┐
                ▼              ▼              ▼
          Host B          Host C          Host D
          nginx-ui        nginx-ui        nginx-ui
          └─► nginx       └─► nginx       └─► nginx
```

## Setup

### 1. Install nginx-ui on every host

Use the official installer or the Docker image — same instance type as the lead.

### 2. Generate a Node Secret on each peer

Log into the peer's Web UI, go to **Settings → Node**, copy the **Node Secret**.

### 3. Register peers on the lead node

Edit the lead's `app.ini`:

```ini
[cluster]
Node = http://10.0.0.2:9000?name=host-b&node_secret=<host-b-secret>&enabled=true
Node = http://10.0.0.3:9000?name=host-c&node_secret=<host-c-secret>&enabled=true
```

Or via environment variables (Docker):

```yaml
services:
  nginx-ui:
    environment:
      - NGINX_UI_CLUSTER_NODE_0=http://10.0.0.2:9000?name=host-b&node_secret=...&enabled=true
```

### 4. Switch nodes from the Web UI

The node switcher in the top bar routes all subsequent operations to the selected node. Each operation happens **locally on that node** — no SSH involved between hosts.

## Combining cluster + host_via_ssh

You can have each cluster peer run host_via_ssh internally — the container manages its own host's nginx, while cluster federation handles cross-host coordination. This is the cleanest topology for "Docker-only" deployments with native nginx on multiple hosts.
