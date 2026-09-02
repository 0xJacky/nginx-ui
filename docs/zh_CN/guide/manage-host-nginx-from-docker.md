# 在 Docker 中管理宿主机 Nginx

当 Nginx UI 运行在 Docker 中，并需要管理同一宿主机上原生安装的 nginx 时，可按本文完成配置。

::: info 前置条件
- 通过 systemd 运行 nginx 的 Linux 宿主机，或通过 `brew services` 运行 nginx 的 macOS 宿主机
- 同一宿主机上已安装 Docker
- Linux：一个专用于 Nginx UI 的非特权用户（示例中使用 `nginxui`）
- macOS：拥有 Homebrew nginx 服务的登录用户
- Nginx UI 管理员已启用两步验证：配置向导需要已验证的两步验证会话，否则会显示**需要两步验证**
:::

在 macOS 上，请在向导的**检测平台**步骤中选择 **macOS（Homebrew）**。Apple Silicon 默认使用 `/opt/homebrew`。在该步骤中，向导会通过 SSH 查询 Homebrew 并解析 `nginx -V`，自动识别已安装的 nginx 版本以及实际的可执行文件、配置、日志、PID 和 Docroot 路径；这也能识别位于 `/usr/local` 的 Intel Homebrew。继续前请确认服务已加载：

```bash
brew services info nginx
```

## 步骤 1：创建非特权用户（仅 Linux）

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` 赋予该用户读取 /var/log 文件（包括 nginx 日志）的权限。

macOS 应使用运行 `brew services` 的现有登录用户，不要创建单独的服务用户。

## 步骤 2：SSH 目标——通过 Nginx UI 生成密钥对

打开**偏好设置 → Nginx → Nginx 控制模式 → 编辑**，选择**通过 SSH 管理宿主机**，再点击**打开 SSH 配置向导**。修改控制模式需要已验证的两步验证会话。

向导包含五个步骤：**SSH 目标**、**信任与测试**、**检测平台**、**访问与安装**、**验证**。下面的步骤按同样的顺序展开。

在 **SSH 目标**步骤中选择私钥来源：

| 来源 | 行为 |
| --- | --- |
| **生成** | Nginx UI 在托管路径 `/etc/nginx-ui/host_key` 生成 ed25519 密钥对。 |
| **使用已有路径** | Nginx UI 读取容器内你指定路径上已存在的私钥。 |
| **粘贴或上传** | 你粘贴私钥内容或选择文件，Nginx UI 以 0600 权限存放到托管路径。 |

所选来源会保存为 `host_key_source` 设置。不支持加密的私钥。

使用生成流程时，点击**生成密钥对**。

复制显示的公钥，格式如下：

```
ssh-ed25519 AAAAC3...generated nginx-ui@generated
```

将其追加到宿主机用户的 authorized_keys 文件：

```bash
sudo mkdir -p /home/nginxui/.ssh
echo 'ssh-ed25519 AAAA...' | sudo tee -a /home/nginxui/.ssh/authorized_keys
sudo chown -R nginxui:nginxui /home/nginxui/.ssh
sudo chmod 700 /home/nginxui/.ssh
sudo chmod 600 /home/nginxui/.ssh/authorized_keys
```

::: warning 主机密钥验证
宿主机 SSH 模式需要使用 `known_hosts` 允许列表。向导显示新指纹时，请先在宿主机或其他可信渠道确认，再信任该密钥。
:::

## 步骤 3：信任与测试——信任主机身份

打开向导的**信任与测试**步骤，点击**扫描主机密钥**。向导会将 SSH 服务端提供的主机密钥与已配置的 `known_hosts` 文件进行比较。

::: warning 信任前请先验证
只有在通过可信来源比对指纹后，才应信任密钥。这个检查用于避免在首次配置或密钥轮换时连接到错误的主机。
:::

可使用以下可信来源：

- 宿主机控制台或服务商控制面板
- 服务器资产清单中已有的指纹记录
- 在宿主机上直接执行命令，例如：

::: code-group

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

```bash [手动扫描]
ssh-keyscan -p 22 host.docker.internal
```

:::

::: details 手动扫描备用方式
如果自动扫描不可用，请在可信终端中执行向导显示的 `ssh-keyscan` 命令。将输出粘贴到**粘贴 ssh-keyscan 输出**，比对指纹后再信任密钥。
:::

::: tip 主机密钥状态
- **unknown_host**：当前还没有为该主机信任任何密钥。
- **new_algorithm**：该主机已有可信密钥，但扫描到了另一种算法。
- **changed**：同一算法的已信任密钥不再匹配。请按安全敏感事件处理。
- **trusted**：扫描到的密钥与 `known_hosts` 匹配。
:::

信任所有扫描到的密钥后，点击**测试 SSH 连接**。连接成功后向导才允许继续。

## 步骤 4：检测平台

在**检测平台**步骤中选择服务管理器（**Linux（systemd）**或 **macOS（Homebrew）**）。向导会通过 SSH 执行 `nginx -V`，并填入 nginx 可执行文件、配置、日志和 PID 路径。

每个路径字段会标记为**自动检测**（与宿主机上报值一致）或**手动覆盖**（被你改过）。手动覆盖的字段会显示检测值以及**恢复检测值**操作。修改 nginx 可执行文件路径后，可点击**根据该可执行文件重新检测路径**，重新执行 `nginx -V` 并刷新配置、日志和 PID 路径。

nginx 可执行文件路径会保存为 `sbin_path`。如果留空，Nginx UI 会回退到服务管理器的默认值（systemd 为 `/usr/sbin/nginx`，launchd 为 `/opt/homebrew/opt/nginx/bin/nginx`），并在保存配置时写入该默认值。下一步生成的 sudoers 允许列表会精确匹配这个解析后的路径。

## 步骤 5：访问与安装——选择文件访问模式

**访问与安装**步骤首先是**文件访问模式**：

| 模式 | 行为 |
| --- | --- |
| **兼容模式（SFTP）** | Nginx UI 完全通过 SSH 和 SFTP 读写宿主机的 nginx 配置和日志。不会把宿主机目录挂载到容器中。 |
| **高性能模式（挂载）** | Nginx UI 从绑定挂载的宿主机目录读取配置和日志。速度更快，但必须按生成的挂载配置重建容器。 |

所选模式会保存为 `host_access_mode` 设置。

::: details SFTP 模式覆盖的范围
- 配置文件、日志文件，以及证书签发或续期时写出的证书文件，都通过 SFTP 在宿主机上读写。
- 容器无法通过 SFTP 监听宿主机文件变化，因此配置索引和证书发现会改为每 30 秒重新扫描一次。在宿主机上绕过 Nginx UI 所做的修改最多需要 30 秒才会显示在界面中。
- **共享配置目录**自检会被跳过，因为容器与宿主机之间没有共享目录。
- 已知限制：证书目录中可导入证书的扫描仍然读取容器文件系统，因此无法发现只存在于宿主机上的证书。如果依赖该扫描，请在 Nginx UI 中签发或续期证书，或改用挂载模式。
:::

## 步骤 6：访问与安装——安装 sudoers 条目（仅 Linux）

**1. 在 nginx 宿主机上**页签会显示一段 sudoers 配置片段。复制后通过以下命令安装：

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

粘贴配置片段后保存并退出。如果语法有误，visudo 会拒绝保存该文件。

Homebrew launchd 服务运行在登录用户域中，不需要 sudoers 条目。

## 步骤 7：访问与安装——配置文件权限

::: details 可选 ACL 命令
如果 nginxui 用户为非 root 用户，请授予其对 /etc/nginx 的写入权限：

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

在 macOS 上，向导会输出 Homebrew 配置和日志路径的读写检查，而不是 Linux ACL 命令。

## 步骤 8：访问与安装——更新容器

**2. 在 Nginx UI 容器中**页签会显示 compose 配置片段、完整的 override 文件和 docker run 命令。该页签只在容器需要修改时出现：使用挂载模式，或者 Linux Docker Engine 需要为 `host.docker.internal` 添加 host-gateway 映射。将片段合并到现有的 `docker-compose.yml` 中。

生成的片段会设置 `NGINX_UI_DISABLE_BUNDLED_NGINX=true`，避免容器在控制宿主机 nginx 时继续启动内置 nginx 服务。

在挂载模式下，片段还会绑定挂载已配置的配置、日志和 PID 目录。在 Linux 与 macOS 预设之间切换后，请重建容器以应用新路径。SFTP 模式下不会添加任何目录挂载。

如果验证检测到的路径与初始预设不同，请返回容器页签重新生成 compose 片段，重建容器后再运行验证。

::: tip 持久化 Nginx UI 数据
请通过 Docker volume 或 bind mount 持久化 `/etc/nginx-ui`。宿主机密钥允许列表默认保存在 `/etc/nginx-ui/known_hosts`，它应在镜像升级和容器重建后继续存在。
:::

```bash
docker compose up -d --force-recreate nginx-ui
```

宿主机和容器的修改都应用后，运行该步骤底部的**配置检查**。它会执行下一步列出的平台和权限检查。

## 步骤 9：验证配置

打开**验证**，点击**运行验证**。主要检查项应通过：

::: tip 预期验证结果

**验证**步骤只运行 nginx 检查：

- ✓ ssh_connect: 通过 SSH 执行 echo ok 成功
- ✓ nginx_test: 配置文件检查通过

平台和权限检查属于**访问与安装**步骤中的**配置检查**，应用片段后你已经运行过：

- ✓ host_platform: Linux host matches systemd
- ✓ systemctl_is_active: 运行中
- ✓ unit_has_execreload: ExecReload 已声明
- ✓ config_dir_writable: /etc/nginx 可访问
- ✓ log_dir_readable: /var/log/nginx/access.log 可读
- ✓ pid_file_present: /var/run/nginx.pid 存在
- ✓ sudo_available: sudo -n true 执行成功
- ✓ sudoers_coverage: 所有必要条目均已配置

`same_host` 和 `known_hosts_persistence` 属于连接检查组，只有在命令行执行 `nginx-ui host-setup test` 时才会运行。

:::

macOS 上，`host_platform` 会报告 Darwin，`launchctl_service_loaded` 取代两个 systemd 检查，sudo 检查会显示不需要 sudo。配置、日志和 PID 检查默认使用 `/opt/homebrew` 路径。

如果 `known_hosts_persistence` 显示为 warning，请检查 Docker volume 或 bind mount。该警告不会阻止保存，但如果 `/etc/nginx-ui` 未被持久化，容器重建后可信主机密钥可能会丢失。

所有检查通过后，点击**保存配置**。

::: tip 自定义控制命令
如果在 Nginx 设置中配置了 `TestConfigCmd`、`ReloadCmd` 或 `RestartCmd`，Nginx UI 会以 SSH 用户身份通过 `/bin/sh -c` 在宿主机上执行这些命令，而不是内置的 systemd 或 launchd 命令。除非生成的 sudoers 条目覆盖了这些命令所调用的程序，否则请保持为空。
:::

## 故障排查

::: details `sudo_available` 报错 "sudo: a password is required"
- 检查 sudoers 文件中是否包含 `NOPASSWD:`，而不仅仅是 `(root)`。
- 检查文件中行末续行符（`\`）是否正确。
:::

::: details `ssh_connect` 报错 "permission denied (publickey)"
- 验证 authorized_keys 文件中的公钥内容、文件所有者及权限是否正确。
- 检查 sshd_config 中是否启用了 `PubkeyAuthentication yes`。
:::

::: details 宿主机 SSH 密钥变更后 `ssh_connect` 失败
主机密钥变更可能是正常操作，例如重建宿主机或轮换 SSH 密钥；也可能表示目标主机错误或存在中间人攻击。只有在确认新指纹后，才替换已信任的密钥。

1. 打开**信任与测试**步骤。
2. 重新扫描主机密钥。
3. 比对向导显示的旧指纹和新指纹。
4. 在宿主机上或通过服务商控制面板验证新指纹。
5. 勾选确认框，然后点击**替换已信任密钥**。

仅在确认不再使用对应 `known_hosts` 条目后，才使用**高级清理**清理。
:::

::: warning `same_host` 警告 "remote host detected"
您的 `host_address` 解析到了不同的机器。SSH 模式**不支持**跨主机使用；请参阅 [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)。
:::

## CLI 参考

生成宿主机 SSH 使用的密钥对：

```bash
nginx-ui host-setup keygen --out /etc/nginx-ui/host_key
```

输出全部配置片段（`--access-mode` 为必填）：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp
```

只输出 Docker 或宿主机侧片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --host
```

需要机器可读输出、完整 compose override 或 docker run 命令时，可以使用 `--json`、`--override` 或 `--docker-run`。

基于当前设置执行验证：

```bash
nginx-ui host-setup test
```

与向导不同，`test` 会运行全部检查组：连接、平台、权限和 nginx。

## 相关文档

- [Nginx 配置参考](config-nginx.md#通过-ssh-控制宿主机-nginx)
- [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)
