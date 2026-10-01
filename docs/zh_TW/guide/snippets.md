---
outline: [2, 3]
---

# 片段

片段把一段 Nginx 設定放在一處統一管理，例如快取規則、安全回應標頭或網站的 PHP 處理設定。網站透過 include 引用片段，因此修改片段會同時改變所有引用它的網站。片段位於 **管理網站 > 片段**。

## 片段的存放位置 {#where-snippets-live}

每個片段都是 Nginx 設定目錄下 `snippets` 目錄中的一個檔案，例如 `/etc/nginx/snippets/static-cache.conf`。許多發行版本來就把片段放在這裡，因此 Nginx 內附的片段（例如 Debian 的 `snippets/fastcgi-php.conf`）也會列出來。

檔名以 `.conf` 結尾、且只包含字母、數字、點、連字號和底線的檔案才算片段。`snippets/plugins` 目錄為保留目錄，不會被處理。

## 使用片段 {#using-a-snippet}

在網站中使用片段有兩種方式：

- **引用。** 把片段頁面顯示的指令加到 `server` 或 `location` 區塊中：

  ```nginx
  location /assets/ {
      include snippets/static-cache.conf;
  }
  ```

  之後片段的每次修改都會作用到這個網站。
- **插入。** 在網站編輯器中開啟配置模板面板，選擇片段並點選 **插入**。片段的內容會被複製到網站中，之後的修改不會作用到它。同一對話框中的 **引用** 會替你加入 include 指令。

儲存片段時會用 `nginx -t` 測試整體設定並重新載入 Nginx。Nginx 拒絕設定時，會保留原來的片段並顯示錯誤。

::: warning 注意
仍被網站引用的片段無法刪除，否則 Nginx 會拒絕設定。片段頁面會列出引用它的檔案。
:::

## 名稱、描述與變數 {#name-description-and-variables}

片段頁面顯示的名稱和描述儲存在檔案頂端的檔案標頭中。標頭的每一行都是註解，因此 Nginx 一律可以引用這個檔案：

```nginx [snippets/static-cache.conf]
# Nginx UI Template Start
# name = "Static file cache"
#
# [description]
# en = "Cache images, scripts and styles for a week"
# Nginx UI Template End

expires 7d;
add_header Cache-Control "public";
```

標頭使用[配置模板](./nginx-ui-template.md)的格式，只是寫成了註解。片段也可以用同樣的方式在其中宣告變數：

```nginx [snippets/hsts.conf]
# Nginx UI Template Start
# name = "HSTS"
#
# [variables.maxAge]
# type = "string"
# name = { en = "Max Age" }
# value = "31536000"
# Nginx UI Template End

add_header Strict-Transport-Security "max-age={{ .maxAge }}" always;
```

帶變數的片段由配置模板面板填寫變數，只能插入使用。Nginx 無法直接引用它，因為 `{{ }}` 預留位置不是 Nginx 設定。變數需要直接編輯檔案標頭來加入（例如在 **管理設定** 中），片段頁面會保留它們。

## 同步到節點 {#synchronizing-to-nodes}

片段頁面的 **同步** 會把所有片段複製到你選擇的節點並保持更新：儲存的片段會複製過去，刪除的片段也會從節點上刪除。你可以選擇是否取代節點上已存在的同名片段。

網站同步到某個節點時，會帶上它引用的片段。這些片段只會在節點上缺少時才建立，絕不會取代節點上已有的副本，因為發行版內附的片段在不同節點上可能本來就應該不同。如需取代，請同步片段本身。

::: tip 提示
片段同步使用的是[使用叢集節點管理多主機 Nginx](./manage-multi-host-nginx-with-cluster.md) 中的目錄部署，因此在 **管理設定** 中，snippets 目錄也會顯示相同的同步目標。
:::
