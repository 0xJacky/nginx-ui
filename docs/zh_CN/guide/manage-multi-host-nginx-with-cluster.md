# 使用集群节点管理多主机 Nginx

::: tip 推荐方式
当您希望通过单个 Nginx UI 控制面板管理多台主机上的 nginx 时，请使用**集群节点**功能，而不是宿主机 SSH 模式。
:::

## 如何选择

| 需求 | host_via_ssh | 集群节点 |
|---|---|---|
| 主机 A 上的容器管理主机 A 上的 nginx | ✓ | ✓（过于复杂） |
| 主机 A 上的容器管理主机 B 上的 nginx | ✗ | ✓ |
| 通过单个 Web 界面查看多台主机的配置/日志 | 否 | ✓ |
| 对等节点不可达时各主机保持自主运行 | 否 | ✓ |

## 推荐拓扑

| 层级 | 作用 | 说明 |
|---|---|---|
| 浏览器 | 打开主节点的 Nginx UI | 日常操作只需要进入一个控制台 |
| 主节点 | 注册对等节点并提供节点切换器 | 也可以管理本机 nginx |
| 对等节点 | 分别运行自己的 Nginx UI 实例 | 每个节点管理同一主机上的 nginx |
| 集群联邦 | 连接主节点与对等节点 | 请求会在当前选中的节点上执行 |

主节点不会通过 SSH 连接到其他主机，而是通过集群节点连接转发操作。如果某个节点以 Docker 方式运行 Nginx UI，并管理同一宿主机上的 nginx，请只在该节点上配置 [在 Docker 中管理宿主机 Nginx](manage-host-nginx-from-docker.md)。

## 配置步骤

### 1. 在每台主机上安装 nginx-ui

建议每个节点尽量使用相同的部署方式。原生 Linux 安装可以执行：

```bash
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install -r https://cloud.nginxui.com/
```

也可以使用 Docker 部署。可参考 [安装脚本](install-script-linux.md) 和 [快速开始](getting-started.md#docker) 了解可用的安装方式。

### 2. 在每个对等节点上生成 Node Secret

登录对等节点的 Web 界面，进入**设置 → 节点**，复制 **Node Secret**。

### 3. 在主节点上注册对等节点

可以通过 `app.ini` 或 Docker 环境变量配置对等节点：

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

### 4. 在 Web 界面中切换节点

顶栏的节点切换器会将后续所有操作路由到所选节点。每个操作都**在该节点本地执行**。主机之间不需要 SSH 通信。

## 集群与 host_via_ssh 组合使用

您可以让每个集群对等节点在内部运行 host_via_ssh。在这种布局中，容器只管理本机 nginx，跨主机协调由集群联邦处理。

另请参阅：

- [Cluster 配置参考](config-cluster.md)
- [在 Docker 中管理宿主机 Nginx](manage-host-nginx-from-docker.md)
