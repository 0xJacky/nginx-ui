# 宿主機 SSH 設定 — 完整指南

本頁介紹如何設定以 Docker 方式執行的 Nginx UI，使其管理安裝在同一宿主機上的 nginx 原生實例。

## 前置條件

- 已安裝 nginx 並透過 systemd 執行的 Linux 宿主機
- 同一宿主機上已安裝 Docker
- 一個專用於 Nginx UI 的非特權使用者（範例中使用 `nginxui`）

## 步驟 1 — 建立非特權使用者

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` 授予該使用者讀取 /var/log 檔案（包括 nginx 日誌）的權限。

## 步驟 2 — 透過 Nginx UI 產生金鑰對

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

## 步驟 3 — 安裝 sudoers 項目

精靈步驟 2b 會顯示一段 sudoers 設定片段。複製後透過以下指令安裝：

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

貼上設定片段後儲存並退出。如果語法有誤，visudo 會拒絕儲存該檔案。

## 步驟 4 — 套用 ACL（選用，適用於非 root 使用者）

如果 nginxui 使用者為非 root 使用者，請授予其對 /etc/nginx 的寫入權限：

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```

## 步驟 5 — 更新 docker-compose 設定

精靈步驟 2a 會顯示一段 compose 設定片段。將其合併到現有的 `docker-compose.yml` 中，然後執行：

```bash
docker compose up -d --force-recreate nginx-ui
```

## 步驟 6 — 驗證

返回精靈步驟 4，點擊**執行驗證**。所有檢查項應全部通過：

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

點擊**儲存**，設定完成。

## 疑難排解

**`sudo_available` 報錯 "sudo: a password is required"**
- 檢查 sudoers 檔案中是否包含 `NOPASSWD:`，而不僅僅是 `(root)`。
- 檢查檔案中行末續行符（`\`）是否正確。

**`ssh_connect` 報錯 "permission denied (publickey)"**
- 驗證 authorized_keys 檔案中的公鑰內容、檔案擁有者及權限是否正確。
- 檢查 sshd_config 中是否啟用了 `PubkeyAuthentication yes`。

**`same_host` 警告 "remote host detected"**
- 您的 `host_address` 解析到了不同的機器。SSH 模式**不支援**跨主機使用；請參閱 [叢集節點跨主機指南](cluster-node-cross-host.md)。
