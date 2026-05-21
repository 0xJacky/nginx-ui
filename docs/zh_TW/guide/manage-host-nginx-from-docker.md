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

請透過 Docker volume 或 bind mount 持久化 `/etc/nginx-ui`。SSH 主機金鑰允許清單預設保存在 `/etc/nginx-ui/known_hosts`，該檔案應在容器重建後繼續存在。

```bash
docker compose up -d --force-recreate nginx-ui
```

## 步驟 6：信任主機身分

開啟設定精靈中的 **Host Identity** 步驟，點擊 **Scan host keys**。精靈會讀取 SSH 服務端提供的主機金鑰，並與已設定的 `known_hosts` 檔案進行比較。

::: warning 信任前請先驗證
Nginx UI 可以從 SSH 服務端收集金鑰，但無法自行證明該金鑰真實可信。點擊 **Trust this key** 或 **Replace trusted key** 前，請先透過可信來源比對指紋。
:::

可使用以下可信來源之一：

- 宿主機控制台或服務商控制面板
- 伺服器資產清單中已有的指紋記錄
- 在宿主機上直接執行指令，例如：

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

如果自動掃描不可用，可以使用精靈中顯示的手動 fallback：

```bash
ssh-keyscan -p 22 host.docker.internal
```

將輸出貼到 **Paste ssh-keyscan output**，確認指紋後再信任金鑰。

::: tip 多個主機金鑰演算法
精靈可以為同一主機記錄多個主機金鑰演算法。如果顯示 **new_algorithm**，請確認該演算法符合預期，並在驗證新指紋後信任它。
:::

## 步驟 7：驗證設定

返回 **Verify** 步驟，點擊**執行驗證**。阻塞性檢查項應通過：

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
- ✓ known_hosts_persistence: `/etc/nginx-ui/known_hosts` 位於建議的持久化資料目錄下

:::

如果 `known_hosts_persistence` 顯示為 warning，請檢查 Docker volume 或 bind mount。該警告不會阻止儲存，但如果 `/etc/nginx-ui` 未被持久化，容器重建後已信任的主機金鑰可能會遺失。

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

::: details 宿主機 SSH 金鑰變更後 `ssh_connect` 失敗
請將主機金鑰變更視為安全敏感事件。在透過可信渠道確認變更前，不要替換已信任的金鑰。

1. 開啟 **Host Identity** 步驟。
2. 重新掃描主機金鑰。
3. 比對精靈顯示的舊指紋和新指紋。
4. 在宿主機上或透過服務商控制面板驗證新指紋。
5. 勾選確認框，然後點擊 **Replace trusted key**。

僅在確認不再使用對應 known_hosts 項目後，才使用 **Advanced cleanup** 清理 stale 項目。
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
