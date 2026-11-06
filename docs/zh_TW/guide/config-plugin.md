# Plugin

- 版本：`>= v3.0.0`

外掛為 Nginx UI 增加功能，例如 DNS-01 憑證所需的 DNS 服務商。外掛在 **系統 > 外掛** 中管理，本節的選項顯示在 **偏好設定 > 外掛** 中。如何撰寫外掛，請參閱 [外掛開發指南](/zh_TW/plugin/overview)。

## 安裝外掛

外掛可以透過以下方式安裝：

- 從外掛頁面的 **市集** 分頁安裝；
- 透過 **從檔案安裝** 上傳安裝套件（`.tar.gz`），或執行 `nginx-ui plugin install <安裝套件>`；
- 離線安裝：把安裝套件複製到外掛目錄下的 `packages` 目錄（預設為 `<設定目錄>/plugins/packages`）。Nginx UI 下次啟動時，如果安裝套件的版本比已安裝的版本新，就會安裝它，然後把安裝套件移到 `packages/installed`。

要為無法連上網際網路的節點準備安裝套件，可以在能連上外掛市集的機器上執行 `nginx-ui plugin fetch <外掛 ID> --platform <os>-<arch>`，例如 `--platform linux-amd64`。使用 `--platform all` 會下載所有平台的安裝套件。

每個安裝套件都帶有簽章，簽署者決定了它的信任等級：

| 等級 | 簽署者 | 安裝條件 |
| --- | --- | --- |
| `official` | Nginx UI 專案 | 直接安裝 |
| `verified` | 專案認可的合作夥伴 | 直接安裝 |
| `community` | 外掛作者 | 開啟 [AllowCommunityPlugins](#allowcommunityplugins) 時，確認後安裝 |
| `unsigned` | 無簽章或未知金鑰 | 僅在 [DeveloperMode](#developermode) 下可以安裝 |

外掛執行前會列出它要求的權限，在你核准之前一直保持停用。詳見 [簽章與信任](/zh_TW/plugin/signing)。

## DNS-01 外掛

從 v3.0.0 起，DNS-01 憑證使用的 DNS 服務商不再內建在 Nginx UI 中，而是由官方外掛 `com.nginxui.dns01` 提供，安裝後約佔用 60 MB。

當有憑證使用 DNS-01 驗證而該外掛尚未安裝時，Nginx UI 會在啟動時以及每天自動安裝它。安裝套件依序從離線 `packages` 目錄、[BundledDir](#bundleddir) 和外掛市集取得，且只接受 `official` 等級的安裝套件。官方 Docker 映像檔內建了該外掛，容器首次啟動時會自動安裝並啟用。

解除安裝該外掛後，除非仍有憑證需要 DNS-01，否則不會被重新安裝。

## Enabled

- 類型：`boolean`
- 預設值：`true`

整個外掛系統的開關。關閉後會停止所有外掛，並隱藏外掛相關頁面。

## Dir

- 類型：`string`
- 預設值：`app.ini` 所在目錄下的 `plugins`

已安裝外掛以及離線安裝用的 `packages` 目錄所在的目錄。在官方 Docker 映像檔中為 `/etc/nginx-ui/plugins`，位於 `/etc/nginx-ui` 磁碟區內。此選項只能在 `app.ini` 中或透過環境變數設定。

## BundledDir

- 類型：`string`
- 預設值：空

發行版隨程式附帶的唯讀外掛安裝套件目錄。每個節點會安裝並啟用其中的外掛一次，並在附帶的安裝套件較新時升級。被解除安裝的附帶外掛不會再次出現。此目錄只會安裝 `official` 等級的安裝套件。官方 Docker 映像檔將其設定為 `/usr/local/share/nginx-ui/plugins`。此選項只能在 `app.ini` 中或透過環境變數設定。

## DefaultSyncPolicy

- 類型：`string`
- 預設值：`manual`
- 可選值：`manual`、`auto`

新安裝外掛的叢集同步策略。`auto` 會讓子節點保持安裝該外掛，並保持版本和啟用狀態一致。

## MarketplaceEnabled

- 類型：`boolean`
- 預設值：`true`

開啟外掛市集。關閉後只能透過檔案和離線目錄安裝外掛。

## MarketplaceSources

- 類型：`[]string`
- 預設值：空

額外外掛目錄的 URL，依合併順序排列。官方目錄 `https://plugins.nginxui.com/v1/index.json` 一律會被讀取，並排在其他目錄之前。目錄來源在外掛頁面的市集分頁中管理。

## TrustedPublicKeys

- 類型：`[]string`
- 預設值：空

可信發布者的 minisign 公鑰，每一項後面可以加上一個空格和發布者名稱。使用這些金鑰簽署的安裝套件以 `community` 等級安裝，上傳或複製到離線目錄的安裝套件也是如此。在偏好設定中顯示為 **可信發布者**。

## AllowCommunityPlugins

- 類型：`boolean`
- 預設值：`true`

允許安裝 `community` 等級的外掛，每次安裝前都需要確認。

## AllowUploads

- 類型：`boolean`
- 預設值：`true`

允許安裝透過瀏覽器或命令列上傳的安裝套件。

## AutoUpdate

- 類型：`boolean`
- 預設值：`false`

在要求的權限不變時，自動更新 `official` 和 `verified` 等級的外掛。

## AllowInsecureDownloadURL

- 類型：`boolean`
- 預設值：`false`

接受明文 `http` 的目錄位址和下載位址。僅用於可信網路中的私有目錄。

## DeveloperMode

- 類型：`boolean`
- 預設值：`false`

允許從任何來源安裝未簽章的外掛。僅在開發外掛時開啟，切勿在正式環境節點上使用。

## MemoryLimitMB

- 類型：`int`
- 預設值：`0`（不限制）

每個外掛程序的記憶體上限，單位 MiB。外掛自身要求的上限較小時，以外掛的為準。僅在使用 cgroup v2 的 Linux 上生效。

## CPUPercent

- 類型：`int`
- 預設值：`0`（不限制）

每個外掛程序的 CPU 上限，以單一核心的百分比表示：`100` 即一個核心。外掛自身要求的上限較小時，以外掛的為準。僅在使用 cgroup v2 的 Linux 上生效。

## CgroupRoot

- 類型：`string`
- 預設值：`/sys/fs/cgroup`

執行資源限制所用的 cgroup v2 掛載點。每個外掛對應 `<CgroupRoot>/nginx-ui/plugins/<id>`。此選項只能在 `app.ini` 中或透過環境變數設定。
