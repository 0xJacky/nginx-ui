---
outline: [2, 3]
---

# 官方外掛目錄

官方外掛目錄位於 `https://plugins.nginxui.com`，是每個 Nginx UI 預設提供的外掛市集。外掛在[開發者中心](https://portal.nginxui.com)提交、審查和管理。本頁介紹這一流程；外掛目錄的格式以及如何架設自己的外掛目錄，參見[外掛目錄](./catalog.md)。

## 提交前的準備 {#before-you-submit}

你的外掛需要：

- 一個公開的 GitHub 儲存庫，至少有一個 GitHub Release，其 tag 是 `v1.0.0` 這樣的語意化版本。Release 的附件就是外掛套件：通用的 `<id>-<version>.tar.gz`、每個平台一個的 `<id>-<version>-<goos>-<goarch>.tar.gz`，或兩者都有，每個外掛套件旁邊附帶一個 `.sha256` 檔案。參見[打包](./packaging.md)。
- 能通過 `nginx-ui plugin lint` 的 `plugin.json`；有伺服器端進程的外掛還要通過 `nginx-ui plugin conformance`。外掛目錄會對你的版本執行這兩項檢查。
- 符合[命名規則](./naming.md)的外掛 ID。沒有自己的網域時，使用 `io.github.<owner>.<name>`。`com.nginxui.` 命名空間保留給 Nginx UI 專案自己的外掛。
- 一把主金鑰以及由它認證的簽章金鑰，使用 `nginx-ui plugin key init --id <外掛 ID>` 建立一次即可；外掛套件用 `nginx-ui plugin pack --key signing.key` 簽署。參見[簽章金鑰](./signing.md#signing-keys)。

## 提交 {#submitting}

1. 使用 GitHub 登入[開發者中心](https://portal.nginxui.com)，選擇**提交**。
2. 選擇儲存庫。儲存庫需要同意被收錄：你需要對它有管理員權限，或者在它上面安裝了 [NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog) GitHub App。
3. 貼上你的主公鑰，即 `primary.pub` 中的 base64 那一行。它會成為目錄項目的 `author_public_key`，決定哪些外掛套件是 `community`，參見[信任等級](./signing.md#trust-levels)。
4. 檢查會在你的最新版本上執行，並同時呈現結果和上架後的展示效果：
   - 項目符合目錄 schema，外掛 ID 唯一；對於 `io.github.<owner>.<name>` 形式的 ID，儲存庫屬於 `<owner>`；
   - 每個外掛套件都與 GitHub 記錄的 sha256 一致，`plugin.sums` 列出了所有檔案，並帶有由你的主金鑰為該外掛認證過的簽章金鑰所做的簽章；
   - 在 linux-amd64 節點會安裝的那個外掛套件上，`lint` 和 `conformance` 能夠通過。
5. 提交。變更頁面會追蹤提交所處的階段（已提交、檢查、審查、已合併、已上線），並顯示目前在等待誰。你可以撤回提交；檢查失敗後，發佈修正版本即可重試。

無法通過驗證的版本不會進入外掛目錄；最新版本通過驗證之前，提交無法上架。

## 審查 {#review}

維護者對每個提交審查一次：

1. 儲存庫同意被收錄。
2. 名稱沒有冒充其他外掛或 Nginx UI 專案。每種語言的名稱都會審查，任何名稱都不能聲稱是官方的。
3. 權限和能力與外掛的功能相符，並且 `permission_reasons` 說明了原因。
4. README 和螢幕截圖與外掛本身相關，並遵守行為準則。
5. 授權條款和分類合適。

審查通過後，外掛目錄會重新建置，外掛會在一小時內出現在每個節點的外掛市集中；使用者手動重新整理外掛市集時會立即出現。

## 上架之後 {#after-listing}

之後的版本不會逐一審查。外掛目錄每小時讀取一次你的 GitHub Releases，新版本的外掛套件一旦通過驗證就會上架。預發佈版本會進入其版本號所指的[發佈通道](./catalog.md#release-channels)，草稿 Release 在發佈之前會被忽略。

如果希望版本在幾分鐘內上架，可以只在外掛儲存庫上安裝 [NGINX UI Plugin Catalog](https://github.com/apps/nginx-ui-plugin-catalog) GitHub App。之後發佈 Release 會立即觸發外掛目錄建置。這個 App 只接收 Release 事件，不持有私鑰，也無法讀取你的儲存庫。

已上架的版本是固定的：在 GitHub 上替換它的某個外掛套件，不會改變節點下載到的內容，只會讓外掛目錄建置失敗。請發佈新版本。

### 版本會改變什麼 {#what-your-releases-change}

上架內容跟隨最新的、未被撤回的穩定版本；還沒有穩定版本時，跟隨最新版本：

| 展示內容 | 來自該版本的 |
| --- | --- |
| 名稱 | `plugin.json` 的 `name` 和 `i18n.<語言>.name`，經維護者核准後生效。 |
| 描述 | `plugin.json` 的 `description` 和 `i18n.<語言>.description`。 |
| 首頁 | `plugin.json` 的 `homepage_url`。 |
| README | 該版本 tag 上的 `README.md`。 |
| 圖示 | 外掛套件中由 `icon_path` 指定的檔案：不超過 256 KB 的 PNG、WebP 或 SVG。 |
| 螢幕截圖 | `plugin.json` 的 `screenshots`，參見[螢幕截圖](./manifest.md#screenshots)。 |

當某個版本在任何語言中使用了上架內容中還沒有的名稱時，上架內容會保留原有名稱，直到維護者核准新名稱。包含「官方」一類字樣或不可見字元的名稱會被忽略。

描述不需要審查，但如果一句話裡同時提到 Nginx UI 並聲稱是官方的，例如「Nginx UI 官方 DNS 外掛」，這條描述會被忽略，上架內容保留原有描述。「Cloudflare 官方 API」這樣的寫法沒有問題。

## 管理上架內容 {#managing-a-listing}

開發者中心裡外掛的頁面包括：

- **商店**、**螢幕截圖**和**翻譯**：外掛市集展示的內容，預覽使用與 Nginx UI 相同的元件繪製。修改會作為變更提交，可以在變更頁面追蹤。
- **版本**：撤回某個版本，或恢復已撤回的版本。已撤回的版本不會再提供給尚未安裝它的節點，正在執行它的節點會收到更新或移除的提示。不要刪除已撤回版本的 GitHub Release：已經安裝它的節點仍需要在外掛目錄中看到它。
- **簽署者**：撤銷可能已經外洩的簽章金鑰。由它簽署的所有版本都會被撤回，節點會把它簽署的外掛套件視為未簽署，所以請用新金鑰簽署並發佈新版本。只是想更換沒有外洩的簽章金鑰時，執行 `nginx-ui plugin key rotate` 即可，外掛目錄無需任何改動。
- **存取權限**：還有誰可以管理這個上架項目。

撤回版本、恢復版本和撤銷簽署者無需審查即可生效。更換主金鑰（用於主金鑰遺失或外洩的情況）需要維護者審查，並且會撤回所有已上架的版本，因為它們的簽署者憑證將不再能通過驗證。

## 合作夥伴外掛 {#partner-plugins}

`verified` 信任等級用於 Nginx UI 維護者擔保的合作夥伴組織。它表明外掛套件由誰發佈，並不表示有人審查過其原始碼。

1. 在開發者中心你的組織頁面提交申請。雙方達成一致後，產生一對 minisign 金鑰（`minisign -G -p partner.pub -s partner.key`），只傳送 `partner.pub`。
2. 維護者把這把公鑰加入簽署的合作夥伴金鑰圈，並寄回一份合作夥伴憑證：`plugin.partner` 和 `plugin.partner.minisig`，其中寫明你的組織，並且可以帶有到期日期。
3. 在寫入 `plugin.sums` 之前，把這兩個檔案原樣放到每個外掛套件的根目錄，然後用你的合作夥伴金鑰為 `plugin.sums` 簽署。

當憑證能用節點固定的官方金鑰驗證，或者節點的金鑰圈副本列出了你的公鑰時，節點會把這樣的外掛套件作為 `verified` 安裝。請在到期日期之前申請新憑證。如果金鑰可能已經外洩，請透過[安全公告](https://github.com/0xJacky/nginx-ui/security/advisories/new)私下回報：維護者會在金鑰圈中撤銷它，節點在下一次重新整理外掛目錄時就不再信任它。

## 已上架外掛的安全問題 {#security-problems-in-a-listed-plugin}

已上架外掛中的漏洞、憑證外洩或授權條款違規，請透過[安全公告](https://github.com/0xJacky/nginx-ui/security/advisories/new)私下回報，不要公開提 issue。對於安全問題，維護者可以不等作者，直接撤回版本或下架外掛，事後再通知作者。
