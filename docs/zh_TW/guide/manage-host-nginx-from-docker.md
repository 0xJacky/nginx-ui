# 在 Docker 中管理宿主機 Nginx

當 Nginx UI 執行在 Docker 中，並需要管理同一宿主機上原生安裝的 nginx 時，可按本文完成設定。

::: info 前置條件
- 透過 systemd 執行 nginx 的 Linux 宿主機，或透過 `brew services` 執行 nginx 的 macOS 宿主機
- 同一宿主機上已安裝 Docker
- Linux：一個專用於 Nginx UI 的非特權使用者（範例中使用 `nginxui`）
- macOS：擁有 Homebrew nginx 服務的登入使用者
:::

在 macOS 上，請在精靈的**偵測平台**步驟中選擇 **macOS (Homebrew)**。Apple Silicon 預設使用 `/opt/homebrew`。在該步驟中，精靈會透過 SSH 查詢 Homebrew 並解析 `nginx -V`，自動識別已安裝的 nginx 版本以及實際的執行檔、設定、日誌、PID 和 Docroot 路徑；這也能識別位於 `/usr/local` 的 Intel Homebrew。繼續前請確認服務已載入：

```bash
brew services info nginx
```

## 步驟 1：建立非特權使用者（僅 Linux）

```bash
sudo useradd -r -s /bin/bash -m -G adm nginxui
```

`-G adm` 授予該使用者讀取 /var/log 檔案（包括 nginx 日誌）的權限。

macOS 應使用執行 `brew services` 的現有登入使用者，不要建立單獨的服務使用者。

## 步驟 2：透過 Nginx UI 產生金鑰對

開啟**偏好設定 → Nginx → Nginx 控制模式 → 編輯**，選擇**透過 SSH 管理宿主機**，然後點擊**開啟 SSH 設定精靈**。變更控制模式需要已驗證的雙因素工作階段。

精靈共有五個步驟：**SSH 目標**、**信任與測試**、**偵測平台**、**安裝**與**驗證**。

在**偵測平台**中，每個路徑欄位在與宿主機回報的值一致時標示為**自動偵測**，被你改動後則標示為**手動覆寫**。被覆寫的欄位會顯示偵測到的值，並提供**還原偵測值**動作。變更 nginx 執行檔路徑後，可使用**依此執行檔重新偵測路徑**再次執行 `nginx -V`，以重新整理設定檔、日誌與 PID 路徑。

在 **SSH 目標**中選擇私鑰來源：

| 來源 | 行為 |
| --- | --- |
| **產生** | Nginx UI 在受管理路徑 `/etc/nginx-ui/host_key` 建立 ed25519 金鑰對。 |
| **既有路徑** | Nginx UI 讀取容器內已存在的金鑰，路徑由你輸入。 |
| **貼上或上傳** | 你貼上金鑰或選擇檔案，Nginx UI 以 0600 權限存放於受管理路徑。 |

所選來源會保存為 `host_key_source` 設定。不支援加密的私鑰。

產生流程請點擊**產生金鑰對**。

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
宿主機 SSH 模式需要使用 `known_hosts` 允許清單。精靈顯示新指紋時，請先在宿主機或其他可信管道確認，再信任該金鑰。
:::

## 步驟 3：安裝 sudoers 項目（僅 Linux）

精靈**安裝**步驟的**宿主機**頁籤會顯示一段 sudoers 設定片段。複製後透過以下指令安裝：

```bash
sudo visudo -f /etc/sudoers.d/nginx-ui
```

貼上設定片段後儲存並退出。如果語法有誤，visudo 會拒絕儲存該檔案。

Homebrew launchd 服務執行於登入使用者域中，不需要 sudoers 項目。

## 步驟 4：設定檔案權限

::: details 選用 ACL 指令
如果 nginxui 使用者為非 root 使用者，請授予其對 /etc/nginx 的寫入權限：

```bash
sudo setfacl -R  -m u:nginxui:rwx /etc/nginx
sudo setfacl -dR -m u:nginxui:rwx /etc/nginx
```
:::

在 macOS 上，精靈會輸出 Homebrew 設定與日誌路徑的讀寫檢查，而不是 Linux ACL 指令。

## 步驟 5：更新 docker-compose 設定

精靈**安裝**步驟的**容器**頁籤會顯示一段 compose 設定片段。將其合併到現有的 `docker-compose.yml` 中。

產生的片段會設定 `NGINX_UI_DISABLE_BUNDLED_NGINX=true`，避免容器在控制宿主機 nginx 時繼續啟動內建 nginx 服務。

片段也會綁定掛載已設定的 PID 目錄。在 Linux 與 macOS 預設之間切換後，請重建容器以套用新路徑。

如果驗證偵測到的路徑與初始預設不同，請返回容器步驟重新產生 compose 片段，重建容器後再執行驗證。

::: tip 持久化 Nginx UI 資料
請透過 Docker volume 或 bind mount 持久化 `/etc/nginx-ui`。宿主機金鑰允許清單預設保存在 `/etc/nginx-ui/known_hosts`，它應在映像升級和容器重建後繼續存在。
:::

macOS 上，`host_platform` 會回報 Darwin，`launchctl_service_loaded` 取代兩個 systemd 檢查，sudo 檢查會顯示不需要 sudo。設定、日誌與 PID 檢查預設使用 `/opt/homebrew` 路徑。

```bash
docker compose up -d --force-recreate nginx-ui
```

## 步驟 6：信任主機身分

開啟設定精靈中的**主機身分**，點擊**掃描主機金鑰**。精靈會將 SSH 服務端提供的主機金鑰與已設定的 `known_hosts` 檔案進行比較。

::: warning 信任前請先驗證
只有在透過可信來源比對指紋後，才應信任金鑰。這個檢查用於避免在首次設定或金鑰輪換時連線到錯誤的主機。
:::

可使用以下可信來源：

- 宿主機控制台或服務商控制面板
- 伺服器資產清單中已有的指紋記錄
- 在宿主機上直接執行指令，例如：

::: code-group

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
```

```bash [手動掃描]
ssh-keyscan -p 22 host.docker.internal
```

:::

::: details 手動掃描備用方式
如果自動掃描不可用，請在可信終端中執行精靈顯示的 `ssh-keyscan` 指令。將輸出貼到**貼上 ssh-keyscan 輸出**，比對指紋後再信任金鑰。
:::

::: tip 主機金鑰狀態
- **unknown_host**：目前還沒有為該主機信任任何金鑰。
- **new_algorithm**：該主機已有可信金鑰，但掃描到了另一種演算法。
- **changed**：同一演算法的已信任金鑰不再匹配。請按安全敏感事件處理。
- **trusted**：掃描到的金鑰與 `known_hosts` 匹配。
:::

## 步驟 7：驗證設定

返回**驗證**，點擊**執行驗證**。主要檢查項應通過：

::: tip 預期驗證結果

**驗證**步驟只執行 nginx 檢查：

- ✓ ssh_connect: 透過 SSH 執行 echo ok 成功
- ✓ nginx_test: 設定檔檢查通過

平台和權限檢查屬於**存取與安裝**步驟中的**安裝檢查**，套用片段後你已經執行過：

- ✓ host_platform: Linux host matches systemd
- ✓ systemctl_is_active: 執行中
- ✓ unit_has_execreload: ExecReload 已宣告
- ✓ config_dir_writable: /etc/nginx 可存取
- ✓ log_dir_readable: /var/log/nginx/access.log 可讀
- ✓ pid_file_present: /var/run/nginx.pid 存在
- ✓ sudo_available: sudo -n true 執行成功
- ✓ sudoers_coverage: 所有必要項目均已設定

`same_host` 和 `known_hosts_persistence` 屬於連線檢查組，只有在命令列執行 `nginx-ui host-setup test` 時才會執行。

:::

如果 `known_hosts_persistence` 顯示為 warning，請檢查 Docker volume 或 bind mount。該警告不會阻止儲存，但如果 `/etc/nginx-ui` 未被持久化，容器重建後可信主機金鑰可能會遺失。

所有檢查通過後，點擊**儲存設定**。

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
主機金鑰變更可能是正常操作，例如重建宿主機或輪換 SSH 金鑰；也可能表示目標主機錯誤或存在中間人攻擊。只有在確認新指紋後，才替換已信任的金鑰。

1. 開啟**信任與測試**步驟。
2. 重新掃描主機金鑰。
3. 比對精靈顯示的舊指紋和新指紋。
4. 在宿主機上或透過服務商控制面板驗證新指紋。
5. 勾選確認框，然後點擊**取代已信任金鑰**。

僅在確認不再使用對應 `known_hosts` 項目後，才使用**進階清理**清理。
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
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp
```

只輸出 Docker 或宿主機側片段：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --compose
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui --access-mode sftp --host
```

需要機器可讀輸出、完整 compose override 或 docker run 指令時，可以使用 `--json`、`--override` 或 `--docker-run`。

基於目前設定執行驗證：

```bash
nginx-ui host-setup test
```

## 相關文件

- [Nginx 設定參考](config-nginx.md#透過-ssh-控制宿主機-nginx)
- [使用叢集節點管理多主機 Nginx](manage-multi-host-nginx-with-cluster.md)
