# 叢集節點 — 跨主機設定

當您希望透過單一 Nginx UI 控制面板管理多台主機上的 nginx 時，正確的工具是**叢集節點**功能，而非宿主機 SSH 模式。

## 如何選擇

| 需求 | host_via_ssh | 叢集節點 |
|---|---|---|
| 主機 A 上的容器管理主機 A 上的 nginx | ✓ | ✓（過於複雜） |
| 主機 A 上的容器管理主機 B 上的 nginx | ✗ | ✓ |
| 透過單一 Web 介面檢視多台主機的設定/日誌 | — | ✓ |
| 對等節點無法連線時各主機保持自主執行 | — | ✓ |

## 建議拓撲

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

## 設定步驟

### 1. 在每台主機上安裝 nginx-ui

使用官方安裝腳本或 Docker 映像——與主節點保持相同的實例類型。

### 2. 在每個對等節點上產生 Node Secret

登入對等節點的 Web 介面，進入**設定 → 節點**，複製 **Node Secret**。

### 3. 在主節點上註冊對等節點

編輯主節點的 `app.ini`：

```ini
[cluster]
Node = http://10.0.0.2:9000?name=host-b&node_secret=<host-b-secret>&enabled=true
Node = http://10.0.0.3:9000?name=host-c&node_secret=<host-c-secret>&enabled=true
```

或透過環境變數設定（Docker 方式）：

```yaml
services:
  nginx-ui:
    environment:
      - NGINX_UI_CLUSTER_NODE_0=http://10.0.0.2:9000?name=host-b&node_secret=...&enabled=true
```

### 4. 在 Web 介面中切換節點

頂列的節點切換器會將後續所有操作路由到所選節點。每個操作都**在該節點本地執行**——主機間無需 SSH 通訊。

## 叢集與 host_via_ssh 組合使用

您可以讓每個叢集對等節點在內部執行 host_via_ssh——容器管理本機上的 nginx，同時由叢集聯邦處理跨主機協調。對於在多台主機上使用原生 nginx 的「純 Docker」部署場景，這是最簡潔的拓撲架構。
