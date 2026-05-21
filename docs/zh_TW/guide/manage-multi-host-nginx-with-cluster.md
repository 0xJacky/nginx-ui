# 使用叢集節點管理多主機 Nginx

::: tip 建議方式
當您希望透過單一 Nginx UI 控制面板管理多台主機上的 nginx 時，請使用**叢集節點**功能，而不是宿主機 SSH 模式。
:::

## 如何選擇

| 需求 | host_via_ssh | 叢集節點 |
|---|---|---|
| 主機 A 上的容器管理主機 A 上的 nginx | ✓ | ✓（過於複雜） |
| 主機 A 上的容器管理主機 B 上的 nginx | ✗ | ✓ |
| 透過單一 Web 介面檢視多台主機的設定/日誌 | 否 | ✓ |
| 對等節點無法連線時各主機保持自主執行 | 否 | ✓ |

## 建議拓撲

| 層級 | 作用 | 說明 |
|---|---|---|
| 瀏覽器 | 開啟主節點的 Nginx UI | 日常操作只需要進入一個控制台 |
| 主節點 | 註冊對等節點並提供節點切換器 | 也可以管理本機 nginx |
| 對等節點 | 分別執行自己的 Nginx UI 實例 | 每個節點管理同一主機上的 nginx |
| 叢集聯邦 | 連接主節點與對等節點 | 請求會在目前選取的節點上執行 |

主節點不會透過 SSH 連線到其他主機，而是透過叢集節點連線轉發操作。如果某個節點以 Docker 方式執行 Nginx UI，並管理同一宿主機上的 nginx，請只在該節點上設定 [在 Docker 中管理宿主機 Nginx](manage-host-nginx-from-docker.md)。

## 設定步驟

### 1. 在每台主機上安裝 nginx-ui

建議每個節點盡量使用相同的部署方式。原生 Linux 安裝可以執行：

```bash
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install
```

也可以使用 Docker 部署。可參考 [安裝指令碼](install-script-linux.md) 和 [快速開始](getting-started.md#docker) 了解可用的安裝方式。

### 2. 在每個對等節點上產生 Node Secret

登入對等節點的 Web 介面，進入**設定 → 節點**，複製 **Node Secret**。

### 3. 在主節點上註冊對等節點

可以透過 `app.ini` 或 Docker 環境變數設定對等節點：

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

### 4. 在 Web 介面中切換節點

頂列的節點切換器會將後續所有操作路由到所選節點。每個操作都**在該節點本地執行**。主機之間不需要 SSH 通訊。

## 叢集與 host_via_ssh 組合使用

您可以讓每個叢集對等節點在內部執行 host_via_ssh。在這種佈局中，容器只管理本機 nginx，跨主機協調由叢集聯邦處理。

另請參閱：

- [Cluster 設定參考](config-cluster.md)
- [在 Docker 中管理宿主機 Nginx](manage-host-nginx-from-docker.md)
