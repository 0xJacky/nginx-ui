# Manage Multi-Host Nginx with Cluster

::: tip Recommended approach
When you want to manage nginx on multiple hosts from a single Nginx UI dashboard, use the **cluster Node** feature instead of host SSH mode.
:::

## When to use what

| Need | host_via_ssh | cluster Node |
|---|---|---|
| Container on host A managing nginx on host A | ✓ | ✓ (overkill) |
| Container on host A managing nginx on host B | ✗ | ✓ |
| One Web UI viewing configs/logs across hosts | No | ✓ |
| Per-host autonomy if peer unreachable | No | ✓ |

## Recommended topology

| Layer | Role | Notes |
|---|---|---|
| Browser | Opens the lead Nginx UI instance | Use one dashboard for daily operation |
| Lead node | Registers peer nodes and provides the node switcher | Can also manage its own local nginx |
| Peer nodes | Run their own Nginx UI instance | Each peer manages nginx on the same host |
| Cluster federation | Connects the lead node to peers | Requests run on the selected node |

The lead node does not SSH into other hosts. It forwards operations through the cluster node connection. If a node runs Nginx UI in Docker and manages nginx installed on the same host, configure [Manage Host Nginx from Docker](manage-host-nginx-from-docker.md) on that node only.

## Setup

### 1. Install nginx-ui on every host

Use the same deployment type on every node when possible. For a native Linux installation, run:

```bash
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install
```

Docker deployments are also supported. See [Install Script](install-script-linux.md) and [Getting Started](getting-started.md#docker) for the available installation methods.

### 2. Generate a Node Secret on each peer

Log into the peer's Web UI, go to **Settings → Node**, copy the **Node Secret**.

### 3. Register peers on the lead node

Configure peer nodes from `app.ini` or Docker environment variables:

::: code-group

```ini [app.ini]
[cluster]
Node = http://10.0.0.2:9000?name=host-b&node_secret=<host-b-secret>&enabled=true
Node = http://10.0.0.3:9000?name=host-c&node_secret=<host-c-secret>&enabled=true
```

```yaml [docker-compose.yml]
services:
  nginx-ui:
    environment:
      - NGINX_UI_CLUSTER_NODE_0=http://10.0.0.2:9000?name=host-b&node_secret=...&enabled=true
```

:::

### 4. Switch nodes from the Web UI

The node switcher in the top bar routes all subsequent operations to the selected node. Each operation happens **locally on that node**. There is no SSH connection between hosts.

## Combining cluster + host_via_ssh

You can have each cluster peer run host_via_ssh internally. In that layout, the container manages nginx on its own host, and cluster federation handles cross-host coordination.

See also:

- [Cluster configuration reference](config-cluster.md)
- [Manage Host Nginx from Docker](manage-host-nginx-from-docker.md)
