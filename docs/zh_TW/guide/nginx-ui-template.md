# 配置模板

Nginx UI Template 提供了一種開箱即用的配置模板機制。在 NgxConfigEditor 中，我們設計了一個可視化界面，使使用者能夠方便地將模板中的配置插入到當前的配置文件中。
在本指南中，我們將介紹這種配置模板的文件格式和語法規則。
配置模板文件存儲在 `template/block` 目錄中，我們歡迎並期待您通過提交 [PR](https://github.com/0xJacky/nginx-ui/pulls) 的形式分享您編寫的配置模板。

::: tip
請注意，每次修改或新增配置文件後，需要重新編譯後端以生效。
:::

## 文件格式

Nginx UI Template 文件由兩部分組成：文件頭部以及具體的 Nginx 配置。

以下是一個關於反向代理的配置模板，我們將以此模板為基礎向您介紹 Nginx UI Template 的文件格式及相關語法。

```nginx configuration
# Nginx UI Template Start
name = "Reverse Proxy"
author = "@0xJacky"
description = { en = "Reverse Proxy Config", zh_CN = "反向代理配置"}

[variables.enableWebSocket]
type = "boolean"
name = { en = "Enable WebSocket", zh_CN = "啟用 WebSocket"}
value = true

[variables.clientMaxBodySize]
type = "string"
name = { en = "Client Max Body Size", zh_CN = "客戶端最大請求內容大小"}
value = "1000m"

[variables.scheme]
type = "select"
name = { en = "Scheme", zh_CN = "協議"}
value = "http"
mask = { http = { en = "HTTP" }, https = { en = "HTTPS" } }

[variables.host]
type = "string"
name = { en = "Host", zh_CN = "主機"}
value = "127.0.0.1"

[variables.port]
type = "string"
name = { en = "Port", zh_CN = "端口"}
value = 9000
# Nginx UI Template End

# Nginx UI Custom Start
{{- if .enableWebSocket }}
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
{{- end }}
# Nginx UI Custom End

if ($host != $server_name) {
    return 404;
}

location / {
        {{ if .enableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{ end }}

        client_max_body_size {{ .clientMaxBodySize }};

        proxy_redirect off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_pass {{ .scheme }}://{{ .host }}:{{ .port }}/;
 }
```

## 文件頭部

文件頭部應包含在 `# Nginx UI Template Start` 和 `# Nginx UI Template End` 之間，並遵循 toml 語法。

文件頭部包含以下欄位：

|           欄位           |                    描述                    |             類型              | 必要 |
|:----------------------:|:----------------------------------------:|:---------------------------:|:--:|
|         `name`         |                  配置的名稱                   |           string            | 是  |
|        `author`        |                    作者                    |           string            | 是  |
|     `description`      |         描述，使用 toml 格式的字典來實現多語言描述         |           toml 字典           | 是  |
| `variables.變量名稱.type`  | 變量類型，目前支持 `boolean`, `string` 和 `select` |           string            | 是  |
| `variables.變量名稱.name`  |      變量顯示的名稱，是一個 toml 格式的字典，用於支持多語言      |           toml 字典           | 是  |
| `variables.變量名稱.value` |                  變量的默認值                  | boolean/string (根據 type 定義) | 否  |
| `variables.變量名稱.mask`  |                  選擇框的選項                  |           toml 字典           | 否  |

示例如下：

```toml
# Nginx UI Template Start
name = "Reverse Proxy"
author = "@0xJacky"
description = { en = "Reverse Proxy Config", zh_CN = "反向代理配置"}

[variables.enableWebSocket]
type = "boolean"
name = { en = "Enable WebSocket", zh_CN = "啟用 WebSocket"}
value = true

[variables.clientMaxBodySize]
type = "string"
name = { en = "Client Max Body Size", zh_CN = "客戶端最大請求內容大小"}
value = "1000m"

[variables.scheme]
type = "select"
name = { en = "Scheme", zh_CN = "協議"}
value = "http"
mask = { http = { en = "HTTP" }, https = { en = "HTTPS" } }

[variables.host]
type = "string"
name = { en = "Host", zh_CN = "主機"}
value = "127.0.0.1"

[variables.port]
type = "string"
name = { en = "Port", zh_CN = "端口"}
value = 9000
# Nginx UI Template End
```

其中，名稱、作者及描述將會以摘要的形式在配置列表中顯示。

![配置列表](/assets/nginx-ui-template/zh_TW/config-template-list.png)

當您點擊「查看」按鈕，界面會顯示一個對話框，如下圖所示。

<img src="/assets/nginx-ui-template/zh_TW/config-ui.png" width="350px" title="配置 Modal" />

下表展示了變量類型與使用者界面元素的關係：

|    類型     | 使用者界面元素 |
|:---------:|:------:|
| `boolean` |   開關   |
| `string`  |  輸入框   |
| `select`  |  選擇框   |


## Nginx 配置
Nginx 配置應該在文件頭部之後提供，這部分將使用 Go 的 `text/template` 庫進行解析。這個庫提供了強大的模板生成能力，包括條件判斷、循環以及複雜的文本處理等。
具體語法可以參考 [Go 文件](https://pkg.go.dev/text/template)。

在頭部中定義的變量可以在這部分中使用，如 `.NoneReferer` 和 `.AllowReferers`。請注意，需要預先在頭部定義變量，才能在這部分中使用。

示例如下：

```nginx configuration
location / {
        {{ if .enableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{ end }}

        client_max_body_size {{ .clientMaxBodySize }};

        proxy_redirect off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Forwarded $proxy_add_forwarded;

        proxy_pass {{ .scheme }}://{{ .host }}:{{ .port }}/;
 }
```

當使用者修改前端的表單後，系統將會根據使用者的輸入和配置模板自動生成新的配置內容。

除了模板頭部定義的變量，我們還提供了宏定義的變量，如下表所示：

|    變量名     |           描述            |
|:----------:|:-----------------------:|
|  HTTPPORT  |     Nginx UI 監聽的端口      |
| HTTP01PORT | 用於 HTTP01 Challenge 的端口 |

上述變量可以直接在配置部分使用，無需在頭部定義。
