---
outline: [2, 3]
---

# 簽章與信任

外掛套件自帶簽章，因此無論它來自外掛目錄、上傳、離線外掛套件目錄還是叢集中的另一個節點，Nginx UI 都能知道是誰發佈了它。簽署者決定外掛套件的**信任等級**，而信任等級決定 Nginx UI 是否以及如何安裝它。

簽章使用 [minisign](https://jedisct1.github.io/minisign/)。

## 為外掛套件簽章 {#signing-a-package}

產生一次金鑰對，並妥善保管私鑰：

```bash
minisign -G -p mydns.pub -s mydns.key
```

然後在外掛套件的其他所有檔案都確定後，為每個外掛套件簽章：

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0.tar.gz --key mydns.key
# 或者直接為已有的外掛套件簽章
nginx-ui plugin sign io.github.example.mydns-1.0.0.tar.gz --key mydns.key
```

簽章會在外掛套件根目錄加入兩個檔案：

| 檔案 | 內容 |
| --- | --- |
| `plugin.sums` | 外掛套件其他所有檔案的 SHA-256。 |
| `plugin.sums.minisig` | `plugin.sums` 的 minisign 簽章。 |

之後對任何檔案的修改都需要重新簽章。同一版本的每個外掛套件都要個別簽章。

### 手動簽章 {#signing-by-hand}

`plugin.sums` 每個一般檔案一行，格式與 `sha256sum` 的輸出相同：

```text
<sha256>  <path>
```

- 64 個小寫十六進位數字、兩個空格，然後是相對於外掛套件根目錄、以 `/` 分隔的路徑；
- 每行以換行字元結尾；不能有歸位字元、空行、位元組順序標記或註解；
- 依路徑的位元組順序排序（`LC_ALL=C sort`），每個路徑只出現一次；
- 列出每個一般檔案（包括空檔案），但不包括根目錄下的 `plugin.sums` 和 `plugin.sums.minisig`。不列出目錄。

然後用 `minisign -S -m plugin.sums` 簽章。兩種 minisign 簽章演算法都可以接受。

## Nginx UI 如何檢查外掛套件 {#how-nginx-ui-checks-a-package}

| 外掛套件包含 | 結果 |
| --- | --- |
| 沒有簽章，或只有兩個檔案中的一個 | 未簽章 |
| Nginx UI 不認識的金鑰所做的簽章 | 未簽章 |
| 無法解析的簽章，或無法用它宣告的金鑰驗證的簽章 | **無效** |
| 有效的簽章，但 `plugin.sums` 與檔案不符 | **無效** |
| 有效的簽章且檔案相符 | 由該金鑰簽署 |

::: danger 警告
無效的外掛套件在簽章之後被修改過。Nginx UI 一律拒絕它，即使在開發者模式下也是如此。`plugin.sums` 恰好列出外掛套件的所有一般檔案及其正確的 SHA-256 時，才算相符。
:::

Nginx UI 會在安裝前展示外掛套件時檢查一次，在安裝時再檢查一次，對任何來源都是如此。外掛目錄中的檢查碼永遠不能取代簽章檢查：它只能說明下載到的是外掛目錄所指的檔案。

## 信任等級 {#trust-levels}

| 簽署者 | 等級 |
| --- | --- |
| Nginx UI 專案的官方外掛簽章金鑰 | `official` |
| 專案擔保且未被撤銷的合作夥伴金鑰 | `verified` |
| 外掛套件所來自的目錄項目的 `author_public_key`、Nginx UI **可信發布者**清單中的金鑰，或者它們認證的簽章金鑰 | `community` |
| 沒有簽章，或簽署者未知 | `unsigned` |

等級排序為 `unsigned` < `community` < `verified` < `official`。外掛目錄和維運人員都不能把金鑰提升到 `community` 以上。外掛目錄項目中的 `trust` 標籤只用於顯示，從不授予等級。

官方外掛簽章金鑰是一把獨立的金鑰，與簽署 Nginx UI 發行版的金鑰分開：它為外掛、合作夥伴憑證和合作夥伴金鑰環簽章，但從不用於驗證 Nginx UI 的升級套件。

各等級允許的內容：

- **`official` 和 `verified`** 正常安裝，並可以自動更新。
- **`community`** 只有在維運人員允許社群外掛時才能安裝，並且需要使用者確認外掛套件是由作者而不是 Nginx UI 專案簽署的。上傳或複製到離線目錄的外掛套件，只有當它的金鑰在可信發布者清單中時才是 `community`。
- **`unsigned`** 只能在開發者模式下安裝。

Nginx UI 會記錄每個已安裝外掛的等級和簽署者的金鑰 ID，並顯示出來。更新的外掛套件等級低於已安裝外掛時屬於降級：自動更新會拒絕它，使用者手動更新時會先收到警告。Nginx UI 自動安裝的外掛（例如 DNS-01 外掛）必須是 `official`。

## 以社群作者身分發佈 {#publishing-as-a-community-author}

大多數外掛都是社群外掛。它們的目錄項目登記作者的**主金鑰**，外掛套件則由主金鑰認證過的**簽章金鑰**簽署。主金鑰離線保存，幾乎不會更換；簽章金鑰可以放在 CI 中，更換時不需要改動外掛目錄。官方外掛目錄只收錄依這種方式簽署的社群外掛套件。

1. 建立金鑰並認證簽章金鑰，參見[簽章金鑰](#signing-keys)。
2. 用簽章金鑰為每個外掛套件簽章。
3. 把主公鑰（`.pub` 檔案中的 base64 那一行）作為 `author_public_key` 放進目錄項目，參見[外掛目錄](./catalog.md#publisher)。從該項目下載、由該金鑰或它認證的簽章金鑰簽署的外掛套件就是 `community`。
4. 不透過外掛目錄安裝你的外掛套件的人，需要把你的主公鑰加入 **偏好設定 > 外掛 > 可信發布者**。

### 簽章金鑰 {#signing-keys}

用簽章金鑰簽署的外掛套件會帶上它的憑證，即外掛套件根目錄下的兩個檔案：

| 檔案 | 內容 |
| --- | --- |
| `plugin.signer` | 簽章金鑰的 minisign 公鑰，與 `.pub` 檔案相同。 |
| `plugin.signer.minisig` | 主金鑰對該檔案的簽章。 |

簽章的可信註解指明這把金鑰可以簽署的那一個外掛：

```text
signer:io.github.example.mydns
```

金鑰和憑證只需建立一次：

```bash
minisign -G -p primary.pub -s primary.key   # primary.key 離線保存並備份
minisign -G -p signing.pub -s signing.key   # CI 用來簽章的金鑰
cp signing.pub plugin.signer
minisign -S -m plugin.signer -x plugin.signer.minisig -s primary.key \
  -t "signer:io.github.example.mydns"
```

在用 `signing.key` 簽章之前，把這兩個檔案原樣複製進每個外掛套件，讓 `plugin.sums` 列出它們。一把簽章金鑰可以為多個外掛簽章，每個外掛各有一份憑證。

當 Nginx UI 信任的該外掛套件的主金鑰能驗證憑證、可信註解指明的是外掛套件本身的外掛，且目錄項目的 `revoked_signers` 中沒有這把簽章金鑰時，Nginx UI 接受這份憑證。這時外掛套件是 `community`，並以主金鑰記錄。憑證驗證失敗本身不會帶來信任：外掛套件依實際簽署它的金鑰判斷。

### 更換與撤銷簽章金鑰 {#replacing-and-revoking-a-signing-key}

例行更換或換了電腦時，新建一把簽章金鑰，用主金鑰認證它，再用它為下一個版本簽章即可。目錄項目不需要改動，舊版本仍然依靠各自攜帶的憑證通過驗證。

簽章金鑰可能已經外洩時，把它的金鑰 ID 加入目錄項目的 `revoked_signers`。Nginx UI 不再信任由它簽署的外掛套件，外掛目錄也會把由它簽署的版本標記為已撤回，正在執行這些版本的節點會收到更新提示。

主金鑰不能用這種方式更換。換成新的 `author_public_key` 後，所有舊版本都不再有可信的簽章，所以請離線保存並備份主金鑰。

## 合作夥伴外掛 {#partner-plugins}

與 Nginx UI 專案合作的組織用自己的金鑰簽章，專案用官方外掛簽章金鑰為這個金鑰擔保。它們的外掛套件在任何 Nginx UI 上都能得到 `verified`，即使該 Nginx UI 從未見過這個合作夥伴。

### 合作夥伴憑證 {#the-partner-certificate}

合作夥伴的外掛套件帶有其金鑰的憑證，即外掛套件根目錄下的兩個檔案：

| 檔案 | 內容 |
| --- | --- |
| `plugin.partner` | 合作夥伴的 minisign 公鑰，格式與 `.pub` 檔案相同。 |
| `plugin.partner.minisig` | 專案官方外掛簽章金鑰對該檔案的簽章。 |

簽章的可信註解寫明合作夥伴的名稱，還可以寫明憑證的最後有效日期：

```text
partner:example-corp
partner:example-corp;expires:2027-09-30
```

名稱使用字母、數字、點和連字號。日期是 UTC 日曆日期：憑證在當天結束前一直有效。專案使用 `nginx-ui plugin certify` 簽發憑證，並為憑證設定到期日期。

合作夥伴在為每個外掛套件簽章之前，把這兩個檔案原樣複製進去，使 `plugin.sums` 列出它們。一張憑證適用於該金鑰簽署的所有外掛套件；續期的憑證是修改過的檔案，需要重新簽章。

憑證滿足以下條件時才會被 Nginx UI 接受：`plugin.partner` 是有效的公鑰，官方外掛簽章金鑰驗證了對它的簽章，註解符合上述格式，憑證未到期，且金鑰未被撤銷。不通過的憑證不提供合作夥伴信任，但它本身永遠不會使外掛套件無效：外掛套件會退回到其他信任來源。

### 合作夥伴金鑰環 {#the-partner-keyring}

專案還在官方外掛目錄旁邊發佈一個簽章的金鑰環 `https://plugins.nginxui.com/v1/partners.json`，列出合作夥伴金鑰和已撤銷的金鑰 ID。Nginx UI 重新整理外掛目錄時會一起重新整理它，把最近一份有效的副本儲存在磁碟上，並且從不接受更舊的版本。金鑰環中列出的金鑰即使沒有憑證也能得到 `verified`；被它撤銷的金鑰無論外掛套件帶有什麼憑證，都不會再得到 `verified`。撤銷不會改變已安裝外掛的等級，但由被撤銷金鑰簽署的更新會作為降級被拒絕。

## 開發者模式 {#developer-mode}

::: warning 注意
開發者模式位於 **偏好設定 > 外掛**，預設關閉。開啟後，Nginx UI 會從所有來源安裝未簽章的外掛套件，並把它們的發佈者標記為未確認。它永遠不會接受無效的外掛套件，不會放寬社群外掛政策，也不會讓未簽章的外掛參與自動更新。只在開發期間開啟它。
:::
