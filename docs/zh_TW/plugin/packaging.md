---
outline: [2, 3]
---

# 打包

外掛以 gzip 壓縮的 tar 封存檔（`.tar.gz`）發佈，`plugin.json` 位於根目錄。一個版本可以只發佈一個**通用**外掛套件，也可以為**每個平台**各發佈一個外掛套件，或者兩者都發佈。

## 建置外掛套件 {#building-a-package}

`nginx-ui plugin pack` 從外掛目錄建置外掛套件，傳入金鑰時還會簽章：

```bash
nginx-ui plugin pack ./mydns io.github.example.mydns-1.0.0-linux-amd64.tar.gz --key mydns.key
```

任何能寫 tar 封存檔的工具都可以，只要結果符合本頁的要求。`nginx-ui plugin lint <外掛套件>` 會檢查它。

## 結構 {#layout}

- `plugin.json` 位於封存檔的根目錄，或者正好在一層目錄之下（即 `tar czf x.tar.gz plugin-dir/` 產生的結構）。
- 根目錄包含 `README.md` 和 `LICENSE`。它們是給安裝外掛的人和審查外掛的人看的：缺少 `README.md` 是錯誤，缺少 `LICENSE` 是警告。每個版本的變更寫在該版本的發佈說明裡，參見[版本](./catalog.md#releases)。
- 外掛套件包含 `server.executables` 宣告的每個檔案。
- 不包含符號連結或硬連結。

## 限制 {#limits}

| 限制 | 值 |
| --- | --- |
| 項目數（檔案和目錄） | 10,000 |
| 檔案解壓縮後的總大小 | 256 MiB |

外掛套件一旦超出限制，Nginx UI 會立即停止解壓縮。為許多平台提供可執行檔的原生外掛很快就會超出大小限制，請改為每個平台發佈一個外掛套件。

## 安全路徑 {#safe-paths}

每個封存檔項目和清單中的每個路徑都必須是**安全的相對路徑**：

- 用 `/` 分隔各段，絕不用 `\`；
- 不能為空，不能包含 NUL 位元組；
- 不能是絕對路徑，既不能是 `/foo`，也不能是 `C:foo` 這樣的 Windows 磁碟機路徑；
- 已經是正規形式：沒有 `.` 段，也沒有 `//`；
- 不能是 `.` 或 `..`，也不能以 `../` 開頭。

Nginx UI 會拒絕含有其他路徑的外掛套件，而不會嘗試修正它們，並確保解壓縮出的任何內容都不會落到外掛目錄之外。

## 單一平台的外掛套件 {#per-platform-packages}

單一平台的外掛套件只包含一個平台的可執行檔。它的檔名以該平台結尾，`server.executables` 也只宣告該平台：

```text
io.github.example.mydns-1.0.0-linux-arm64.tar.gz
```

```json [plugin.json]
"server": { "executables": { "linux-arm64": "dist/linux-arm64/mydns" } }
```

同一版本的所有外掛套件，除 `server.executables` 外清單都相同，除可執行檔和簽章檔案外檔案也都相同，因此無論使用者的平台拿到哪個外掛套件，他們核准的內容都是一樣的。

通用外掛套件可以宣告任意多個平台，並包含它宣告的每個平台的可執行檔。沒有 `server`，或使用直譯式 `server.command` 的外掛套件可以在所有平台上執行。

檔名的構成參見[命名規則](./naming.md#package-file-names)。

## 可執行檔 {#executables}

解壓縮後，Nginx UI 會為 `server.executables` 或 `server.command` 中的路徑所指向的每個檔案加上執行權限（無論封存檔中如何記錄），其他檔案的權限位元保持不變。外掛套件不提供的平台應從 `server.executables` 中省略，而不是宣告了卻缺少檔案。

## 可重現的建置 {#reproducible-builds}

在工具鏈允許的範圍內，請以確定的方式建置外掛套件：檔案順序穩定，除格式需要外不嵌入時間戳記。這樣外掛套件的檢查碼才有意義，任何人都能確認外掛套件是從公開的原始碼建置的。如果 `plugin.json` 是產生的，請依鍵排序寫出。

## 安裝 {#installing}

安裝外掛套件時，Nginx UI 會：

1. 在上述限制內解壓縮；
2. 驗證清單，失敗則捨棄全部內容；
3. 檢查[簽章](./signing.md)並得出信任等級；
4. 向使用者顯示外掛的 ID、版本、權限和信任等級，並請求核准；
5. 安裝它，如果之前有舊版本則取代。
