# 在 Docker 中管理宿主機 Nginx

當 Nginx UI 執行在 Docker 中，並需要管理同一宿主機上原生安裝的 nginx 時，可按本文完成設定。

::: info 前置條件
- 已安裝 nginx 並透過 systemd 執行的 Linux 宿主機
- 同一宿主機上已安裝 Docker
- 一個專用於 Nginx UI 的非特權使用者（範例中使用 `nginxui`）
:::

## 步驟 1：建立非特權使用者

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` 授予該使用者讀取 /var/log 檔案（包括 nginx 日誌）的權限。

## 步驟 2：透過 Nginx UI 產生金鑰對

開啟**偏好設定 → Nginx → 透過 SSH 管理宿主機 → 開啟設定精靈**。在步驟 1 中點擊**產生金鑰對**。

複製顯示的公鑰，格式如下：

```
ssh-ed25519 AAAAC3...generated nginx-ui@generated
```

將其附加到宿主機使用者的 authorized_keys 檔案：

```bash
sudo mkdir -p /home/nginxui/.ssh
echo 'ssh-ed25519 AAAA...' | sudo tee -a /home/nginxui/.ssh/authorized_keys
sudo chown -R nginxui:nginxui /home/nginxui/.ssh
sudo chmod 700 /home/nginxui/.ssh
sudo chmod 600 /home/nginxui/.ssh/authorized_keys
```

::: warning 主機金鑰驗證
主機金鑰檢查一律使用已設定的 known_hosts 允許清單。如果設定精靈顯示新的主機指紋，請先確認指紋再信任該金鑰。
:::

## 步驟 3：安裝 sudoers 項目

精靈步驟 2b 會顯示一段 sudoers 設定片段。複製後透過以下指令安裝：

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

貼上設定片段後儲存並退出。如果語法有誤，visudo 會拒絕儲存該檔案。

## 步驟 4：為非 root 使用者套用 ACL

::: details 選用 ACL 指令
如果 nginxui 使用者為非 root 使用者，請授予其對 /etc/nginx 的寫入權限：

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

## 步驟 5：更新 docker-compose 設定

精靈步驟 2a 會顯示一段 compose 設定片段。將其合併到現有的 `docker-compose.yml` 中，然後執行：

產生的片段會設定 `NGINX_UI_DISABLE_BUNDLED_NGINX=true`，避免容器在控制宿主機 nginx 時繼續啟動內建 nginx 服務。

```bash
docker compose up -d --force-recreate nginx-ui
```

## 步驟 6：驗證設定

返回精靈步驟 4，點擊**執行驗證**。所有檢查項應全部通過：

::: tip 預期驗證結果

- ✓ same_host: machine-id 匹配
- ✓ ssh_connect: 透過 SSH 執行 echo ok 成功
- ✓ sudo_available: sudo -n true 執行成功
- ✓ sudoers_coverage: 所有必要項目均已設定
- ✓ systemctl_is_active: 執行中
- ✓ unit_has_execreload: ExecReload 已宣告
- ✓ nginx_test: 設定檔檢查通過
- ✓ config_dir_writable: /etc/nginx 可存取
- ✓ log_dir_readable: /var/log/nginx/access.log 可讀
- ✓ pid_file_present: /var/run/nginx.pid 存在

:::

所有檢查通過後，點擊**儲存**。

## 疑難排解

::: details `sudo_available` 報錯 "sudo: a password is required"
- 檢查 sudoers 檔案中是否包含 `NOPASSWD:`，而不僅僅是 `(root)`。
- 檢查檔案中行末續行符（`\`）是否正確。
:::

::: details `ssh_connect` 報錯 "permission denied (publickey)"
- 驗證 authorized_keys 檔案中的公鑰內容、檔案擁有者及權限是否正確。
- 檢查 sshd_config 中是否啟用了 `PubkeyAuthentication yes`。
:::

::: warning `same_host` 警告 "remote host detected"
您的 `host_address` 解析到了不同的機器。SSH 模式**不支援**跨主機使用；請參閱 [使用叢集節點管理多主機 Nginx](manage-multi-host-nginx-with-cluster.md)。
:::

## CLI 參考

產生宿主機 SSH 使用的金鑰對：

```bash
nginx-ui host-setup keygen --out /etc/nginx-ui/host_key
```

輸出全部設定片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui
```

只輸出 Docker 或宿主機側片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --host
```

需要機器可讀輸出、完整 compose override 或 docker run 指令時，可以使用 `--json`、`--override` 或 `--docker-run`。

基於目前設定執行驗證：

```bash
nginx-ui host-setup test
```

## 相關文件

- [Nginx 設定參考](config-nginx.md#透過-ssh-控制宿主機-nginx)
- [使用叢集節點管理多主機 Nginx](manage-multi-host-nginx-with-cluster.md)
