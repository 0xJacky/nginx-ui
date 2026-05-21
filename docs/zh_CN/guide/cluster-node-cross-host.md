# 集群节点 — 跨主机配置

当您希望通过单个 Nginx UI 控制面板管理多台主机上的 nginx 时，正确的工具是**集群节点**功能，而非宿主机 SSH 模式。

## 如何选择

| 需求 | host_via_ssh | 集群节点 |
|---|---|---|
| 主机 A 上的容器管理主机 A 上的 nginx | ✓ | ✓（过于复杂） |
| 主机 A 上的容器管理主机 B 上的 nginx | ✗ | ✓ |
| 通过单个 Web 界面查看多台主机的配置/日志 | — | ✓ |
| 对等节点不可达时各主机保持自主运行 | — | ✓ |

## 推荐拓扑

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

## 配置步骤

### 1. 在每台主机上安装 nginx-ui

使用官方安装脚本或 Docker 镜像——与主节点保持相同的实例类型。

### 2. 在每个对等节点上生成 Node Secret

登录对等节点的 Web 界面，进入**设置 → 节点**，复制 **Node Secret**。

### 3. 在主节点上注册对等节点

编辑主节点的 `app.ini`：

```ini
[cluster]
Node = http://10.0.0.2:9000?name=host-b&node_secret=<host-b-secret>&enabled=true
Node = http://10.0.0.3:9000?name=host-c&node_secret=<host-c-secret>&enabled=true
```

或通过环境变量配置（Docker 方式）：

```yaml
services:
  nginx-ui:
    environment:
      - NGINX_UI_CLUSTER_NODE_0=http://10.0.0.2:9000?name=host-b&node_secret=...&enabled=true
```

### 4. 在 Web 界面中切换节点

顶栏的节点切换器会将后续所有操作路由到所选节点。每个操作都**在该节点本地执行**——主机间无需 SSH 通信。

## 集群与 host_via_ssh 组合使用

您可以让每个集群对等节点在内部运行 host_via_ssh——容器管理本机上的 nginx，同时由集群联邦处理跨主机协调。对于在多台主机上使用原生 nginx 的"纯 Docker"部署场景，这是最简洁的拓扑架构。
