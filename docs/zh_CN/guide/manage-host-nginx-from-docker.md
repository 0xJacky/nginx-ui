# 在 Docker 中管理宿主机 Nginx

当 Nginx UI 运行在 Docker 中，并需要管理同一宿主机上原生安装的 nginx 时，可按本文完成配置。

::: info 前置条件
- 已安装 nginx 并通过 systemd 运行的 Linux 宿主机
- 同一宿主机上已安装 Docker
- 一个专用于 Nginx UI 的非特权用户（示例中使用 `nginxui`）
:::

## 步骤 1：创建非特权用户

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` 赋予该用户读取 /var/log 文件（包括 nginx 日志）的权限。

## 步骤 2：通过 Nginx UI 生成密钥对

打开**偏好设置 → Nginx → 通过 SSH 管理宿主机 → 打开配置向导**。在步骤 1 中点击**生成密钥对**。

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
主机密钥检查始终使用已配置的 known_hosts 允许列表。如果配置向导显示新的主机指纹，请先确认指纹再信任该密钥。
:::

## 步骤 3：安装 sudoers 条目

向导步骤 2b 会显示一段 sudoers 配置片段。复制后通过以下命令安装：

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

粘贴配置片段后保存并退出。如果语法有误，visudo 会拒绝保存该文件。

## 步骤 4：为非 root 用户应用 ACL

::: details 可选 ACL 命令
如果 nginxui 用户为非 root 用户，请授予其对 /etc/nginx 的写入权限：

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

## 步骤 5：更新 docker-compose 配置

向导步骤 2a 会显示一段 compose 配置片段。将其合并到现有的 `docker-compose.yml` 中，然后执行：

生成的片段会设置 `NGINX_UI_DISABLE_BUNDLED_NGINX=true`，避免容器在控制宿主机 nginx 时继续启动内置 nginx 服务。

请通过 Docker volume 或 bind mount 持久化 `/etc/nginx-ui`。SSH 主机密钥允许列表默认保存在 `/etc/nginx-ui/known_hosts`，该文件应在容器重建后继续存在。

```bash
docker compose up -d --force-recreate nginx-ui
```

## 步骤 6：信任主机身份

打开配置向导中的 **Host Identity** 步骤，点击 **Scan host keys**。向导会读取 SSH 服务端提供的主机密钥，并与已配置的 `known_hosts` 文件进行比较。

::: warning 信任前请先验证
Nginx UI 可以从 SSH 服务端收集密钥，但无法自行证明该密钥真实可信。点击 **Trust this key** 或 **Replace trusted key** 前，请先通过可信来源比对指纹。
:::

可使用以下可信来源之一：

- 宿主机控制台或服务商控制面板
- 服务器资产清单中已有的指纹记录
- 在宿主机上直接执行命令，例如：

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

如果自动扫描不可用，可以使用向导中显示的手动 fallback：

```bash
ssh-keyscan -p 22 host.docker.internal
```

将输出粘贴到 **Paste ssh-keyscan output**，确认指纹后再信任密钥。

::: tip 多个主机密钥算法
向导可以为同一主机记录多个主机密钥算法。如果显示 **new_algorithm**，请确认该算法符合预期，并在验证新指纹后信任它。
:::

## 步骤 7：验证配置

返回 **Verify** 步骤，点击**运行验证**。阻塞性检查项应通过：

::: tip 预期验证结果

- ✓ same_host: machine-id 匹配
- ✓ ssh_connect: 通过 SSH 执行 echo ok 成功
- ✓ sudo_available: sudo -n true 执行成功
- ✓ sudoers_coverage: 所有必要条目均已配置
- ✓ systemctl_is_active: 运行中
- ✓ unit_has_execreload: ExecReload 已声明
- ✓ nginx_test: 配置文件检查通过
- ✓ config_dir_writable: /etc/nginx 可访问
- ✓ log_dir_readable: /var/log/nginx/access.log 可读
- ✓ pid_file_present: /var/run/nginx.pid 存在
- ✓ known_hosts_persistence: `/etc/nginx-ui/known_hosts` 位于推荐的持久化数据目录下

:::

如果 `known_hosts_persistence` 显示为 warning，请检查 Docker volume 或 bind mount。该警告不会阻止保存，但如果 `/etc/nginx-ui` 未被持久化，容器重建后已信任的主机密钥可能会丢失。

所有检查通过后，点击**保存**。

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
请将主机密钥变更视为安全敏感事件。在通过可信渠道确认变更前，不要替换已信任的密钥。

1. 打开 **Host Identity** 步骤。
2. 重新扫描主机密钥。
3. 比对向导显示的旧指纹和新指纹。
4. 在宿主机上或通过服务商控制面板验证新指纹。
5. 勾选确认框，然后点击 **Replace trusted key**。

仅在确认不再使用对应 known_hosts 条目后，才使用 **Advanced cleanup** 清理 stale 条目。
:::

::: warning `same_host` 警告 "remote host detected"
您的 `host_address` 解析到了不同的机器。SSH 模式**不支持**跨主机使用；请参阅 [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)。
:::

## CLI 参考

生成宿主机 SSH 使用的密钥对：

```bash
nginx-ui host-setup keygen --out /etc/nginx-ui/host_key
```

输出全部配置片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui
```

只输出 Docker 或宿主机侧片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --host
```

需要机器可读输出、完整 compose override 或 docker run 命令时，可以使用 `--json`、`--override` 或 `--docker-run`。

基于当前设置执行验证：

```bash
nginx-ui host-setup test
```

## 相关文档

- [Nginx 配置参考](config-nginx.md#通过-ssh-控制宿主机-nginx)
- [使用集群节点管理多主机 Nginx](manage-multi-host-nginx-with-cluster.md)
