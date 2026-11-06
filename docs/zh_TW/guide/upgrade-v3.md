# 升級到 v3

依原本的安裝方式正常升級即可，其餘遷移由 Nginx UI v3 在首次啟動時完成。本頁說明磁碟上的變化，以及無法連上網際網路的節點需要注意的事項。

## DNS-01 憑證

DNS-01 憑證使用的 DNS 服務商已從 Nginx UI 中移出，改由官方外掛 `com.nginxui.dns01` 提供。因此 Nginx UI 的執行檔更小，閒置時使用的記憶體也更少。

不需要任何操作。節點上有 DNS-01 憑證時，Nginx UI 會在啟動時自動安裝並啟用該外掛，之後每天再檢查一次。安裝套件依以下順序尋找：

1. 離線安裝套件目錄 `<設定目錄>/plugins/packages`；
2. 發行版附帶的安裝套件。官方 Docker 映像檔內建了該外掛，無論是否有 DNS-01 憑證，容器首次啟動時都會安裝它；
3. 外掛市集。

外掛安裝完成後，先前等待該外掛的憑證會立即續期。如果無法安裝，Nginx UI 會發出通知，在外掛安裝之前 DNS-01 憑證不會續期。

DNS 憑據會保留。[RecursiveNameservers](./config-cert.md#recursivenameservers) 的值會在外掛首次安裝時複製到外掛設定中。

### 無法連上網際網路的節點

如果節點執行的是官方 Docker 映像檔，內建的外掛就足夠了。否則，請在一台能連上網際網路並安裝了 Nginx UI v3 的機器上下載安裝套件：

```shell
nginx-ui plugin fetch com.nginxui.dns01 --platform linux-amd64
```

平台請填入目標節點的 `<os>-<arch>`，例如 `linux-arm64`。把下載的 `.tar.gz` 檔案複製到節點的 `<設定目錄>/plugins/packages` 中，然後啟動或重新啟動 Nginx UI。也可以在 **系統 > 外掛** 中透過 **從檔案安裝** 上傳安裝套件。

## 日誌索引

日誌分析仍然內建在 Nginx UI 中。索引從 Bleve 改為欄式儲存，佔用的磁碟空間小得多。如果開啟了索引，v3 首次啟動時會從索引目錄（預設為 `app.ini` 同一目錄下的 `log-index`）中刪除舊的索引目錄，並把所有日誌重新索引到其中的 `logstore` 子目錄。重建期間，搜尋、儀表板和地圖的資料會逐步補齊。詳見 [Nginx Log](./config-nginx-log.md)。

## 外掛目錄

已安裝的外掛存放在外掛目錄中，預設為 `app.ini` 同一目錄下的 `plugins`：

| 安裝方式 | 外掛目錄 |
| --- | --- |
| Docker | `/etc/nginx-ui/plugins`，位於 `/etc/nginx-ui` 磁碟區內 |
| Linux 安裝指令碼 | `/usr/local/etc/nginx-ui/plugins` |
| 其他 | `app.ini` 所在目錄下的 `plugins` |

請把這個目錄與設定目錄的其他內容一起保留。外掛相關選項請參閱 [Plugin](./config-plugin.md)。
