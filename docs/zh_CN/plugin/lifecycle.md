---
outline: [2, 3]
---

# 生命周期

本页跟随插件进程从启动到退出的全过程。它适用于带有 `server` 块的插件，没有 `server` 块的插件没有进程。

Nginx UI 用六个方法驱动生命周期：

| 方法 | 类型 | 用途 |
| --- | --- | --- |
| `plugin.initialize` | 请求 | 握手：版本、能力、设置、权限。 |
| `plugin.initialized` | 通知 | 握手成功，插件可以调用宿主 API。 |
| `plugin.configure` | 请求 | 设置已更改。 |
| `plugin.ping` | 请求 | 健康检查。 |
| `plugin.shutdown` | 请求 | 完成进行中的工作，准备退出。 |
| `plugin.exit` | 通知 | 立即退出。 |

::: tip 提示
SDK 实现了所有这些方法，你只需提供设置变更和关闭时要做的事。
:::

## 握手 {#handshake}

启动进程后，Nginx UI 首先发送 `plugin.initialize`：

```json
{
  "jsonrpc": "2.0", "id": 1, "method": "plugin.initialize",
  "params": {
    "host": { "version": "2.7.0", "os": "linux", "arch": "amd64", "locale": "en" },
    "settings": {},
    "permissions": ["network"]
  }
}
```

- `settings` 是插件已保存的设置，首次安装时为 `{}`。
- `permissions` 是插件持有的权限。它总是与清单一致，因为 Nginx UI 不会启动权限尚待批准的插件。

插件用它实现的内容作答：

```json
{ "jsonrpc": "2.0", "id": 1, "result": { "api_version": 1, "capabilities": ["dns01"] } }
```

| 字段 | 含义 |
| --- | --- |
| `api_version` | 进程的协议代次，必须等于 Nginx UI 实现的代次 `1`。 |
| `capabilities` | 运行中的进程实现的能力。作为集合必须与清单的 `capabilities` 完全一致。 |
| `transports` | 插件提供的传输方式，例如 `["stdio", "grpc"]`。标准输入输出总是可用；列出 `grpc` 即启用 [gRPC 传输](./protocol.md#grpc-transport)。 |
| `rpc_socket`、`rpc_pipe`、`rpc_port`、`rpc_token` | gRPC 传输的监听位置，参见[通信协议](./protocol.md#grpc-transport)。 |
| `http_pipe`、`http_port` | Windows 上的插件提供 HTTP 接口的位置，参见 [HTTP 接口](./http.md#where-to-listen)。 |

`api_version` 为其他值，或者能力列表与清单有任何差异，握手都会失败。Nginx UI 最多等待 **10 秒**，因此请把启动时的工作（加载大型目录、预热缓存）控制在这个时间内，或者放到握手之后进行。

握手成功后，Nginx UI 发送 `plugin.initialized`。从这时起插件才可以调用[宿主 API](./host-api.md)，更早的调用可能会被拒绝。

## 设置 {#settings}

插件运行期间，每当用户保存插件的设置，Nginx UI 就会发送新的设置：

```json
{ "jsonrpc": "2.0", "id": 2, "method": "plugin.configure", "params": { "settings": { "timeout": 90 } } }
```

插件在后续调用使用新值之后回复 `{}`。握手时的设置不会作为 `plugin.configure` 调用重复发送，不过 Nginx UI 可能在握手后立即发送一次。`host.settings.get` 可以随时返回当前的设置。

## 健康检查 {#health-checks}

进程运行期间，Nginx UI 每 **15 秒**发送一次 `plugin.ping`，并要求在 **5 秒**内收到 `{}`。连续 **3** 次没有回应，它就会终止进程，并按崩溃处理。

::: warning 注意
插件即使在忙于其他调用时也必须回应 ping。请独立于耗时工作处理 ping，至少也要优先处理它。SDK 会替你做到这一点。
:::

## 停止 {#stopping}

插件被禁用、卸载、更新、（`on_demand` 插件）空闲足够久，或 Nginx UI 关闭时，Nginx UI 会停止插件：

1. 发送 `plugin.shutdown`，最多等待 **5 秒**的回复。插件应利用这一步完成进行中的调用（包括未结束的 gRPC 调用和日志流），并拒绝新的调用，然后回复 `{}`。
2. 发送 `plugin.exit`，无论是否收到了回复。
3. 最多等待 **5 秒**让进程退出。
4. 终止进程。

收到 `plugin.exit` 后，插件应尽快退出，不再等待输入，并顺带关闭 gRPC 和 HTTP 监听，以状态码 `0` 退出。任何其他状态码，或者没有经过这些消息就退出，都视为崩溃。

## 崩溃与重启 {#crashes-and-restarts}

自行退出的 `resident` 进程视为崩溃。Nginx UI 会在 1、2、4、8 秒后以及之后每 16 秒重启它。5 分钟内崩溃 3 次后，它就不再尝试，并把插件标记为失败，直到有人手动重启。

`on_demand` 进程在首次需要时启动，在 `idle_timeout_seconds` 内没有调用后停止，下一次调用会再次启动它。没有人等待它时发生的崩溃不计入限制。

::: warning 注意
进程也可能在任何时刻不经停止流程就被终止，例如超出内存限制时。请不要依赖 `plugin.shutdown` 来保持数据目录中文件的一致性：先写入临时文件再重命名，或者使用能在崩溃后恢复的数据库。
:::

## 环境变量 {#environment}

Nginx UI 为进程设置以下变量：

| 变量 | 含义 |
| --- | --- |
| `NGINX_UI_PLUGIN_ID` | 插件 ID。 |
| `NGINX_UI_PLUGIN_API_VERSION` | Nginx UI 实现的协议代次，例如 `1`。 |
| `NGINX_UI_PLUGIN_DATA_DIR` | 只有本插件可以写入的目录的绝对路径。所有状态都保存在这里。 |
| `NGINX_UI_VERSION` | Nginx UI 的版本。 |
| `NGINX_UI_DEMO` | Nginx UI 作为公开演示运行时为 `1`，插件可以用占位内容代替演示环境无法提供的数据。否则不设置。 |
| `NGINX_UI_PLUGIN_HTTP_SECRET` | 提供 HTTP 接口的插件所用的密钥。参见 [HTTP 接口](./http.md)。 |

### 出站代理 {#outbound-proxy}

当 Nginx UI 配置了出站 HTTP 代理时，持有 `network` 权限的插件会通过 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY`（及其小写形式）收到代理设置，大多数语言的标准 HTTP 客户端无需额外代码就会使用它。`NO_PROXY` 总是包含回环地址。这些变量在进程启动时读取，因此代理设置变更后，插件在下次启动时才会生效。没有 `network` 权限的插件永远不会收到它们。

## 资源限制 {#resource-limits}

在使用 cgroup v2 的 Linux 上，Nginx UI 可以限制插件进程的内存和 CPU 时间。运维人员在 **偏好设置 > 插件** 中设置限制，插件的 [`server.resources`](./manifest.md#resources) 提示值可以降低限制，但不能提高限制。每个插件运行在自己的组 `<cgroup 根目录>/nginx-ui/plugins/<插件 ID>` 中，并禁用交换空间。

超出内存限制的进程会被立即终止并按崩溃处理，因此持续超出限制的插件最终会被标记为失败。在无法强制执行限制的环境中（其他操作系统、权限不足、未委派 cgroup 的容器），插件在没有限制的情况下运行，其详情会说明限制未生效。

## 冲突的插件 {#conflicting-plugins}

两个[冲突](./manifest.md#dependencies-and-conflicts)的插件永远不会同时运行：

- 冲突的插件已启用时，启用插件会失败，错误信息会指出另一个插件，除非用户选择替换它（先禁用它）。
- 一步完成安装和启用的插件包，在冲突的插件已启用时保持禁用，除非选择了替换。
- 启动时，与保持启用的插件冲突的插件会被禁用，并记录一条指出另一个插件的错误。
