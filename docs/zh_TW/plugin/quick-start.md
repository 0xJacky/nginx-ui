---
outline: [2, 3]
---

# 快速上手

本頁建立一個 DNS-01 外掛，建置它並安裝到本機的 Nginx UI 上，大約需要十分鐘。

## 準備工作 {#prerequisites}

- 一個你有管理權限的 Nginx UI，最好是測試環境。
- 撰寫外掛所用語言的工具鏈：Go、Rust、Python 3 或 Node.js。
- 本機上的 `nginx-ui` 可執行檔，用於它的外掛工具。

## 建立外掛 {#create-the-plugin}

`nginx-ui plugin init` 會建立一個可以直接執行的外掛儲存庫：

::: code-group

```bash [Go]
nginx-ui plugin init mydns \
  --id io.github.example.mydns \
  --name "MyDNS" \
  --lang go
```

```bash [Rust]
nginx-ui plugin init mydns \
  --id io.github.example.mydns \
  --name "MyDNS" \
  --lang rust
```

```bash [Python]
nginx-ui plugin init mydns \
  --id io.github.example.mydns \
  --name "MyDNS" \
  --lang python
```

```bash [Node.js]
nginx-ui plugin init mydns \
  --id io.github.example.mydns \
  --name "MyDNS" \
  --lang node
```

:::

| 參數 | 意義 |
| --- | --- |
| `--id` | 外掛 ID，一個反向網域名稱，例如 `io.github.<你的 GitHub 使用者名稱>.<外掛名稱>`。參見[命名規則](./naming.md)。 |
| `--name` | 介面中顯示的名稱。 |
| `--lang` | `go`、`rust`、`python` 或 `node`。 |
| `--capability` | 起始能力。目前只支援 `dns01`。 |

每種語言都會產生一個可以直接使用的專案結構：

::: code-group

```text [Go]
mydns/
├── plugin.json      # 清單檔案
├── main.go          # 基於 plugin-sdk-go 的外掛
├── go.mod
├── build.sh         # 建置 dist/<平台>/mydns，即 plugin.json 指向的可執行檔
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Rust]
mydns/
├── plugin.json      # 清單檔案
├── Cargo.toml
├── src/
│   └── main.rs      # 基於 plugin-sdk-rust 的外掛
├── build.sh         # 建置 dist/<平台>/mydns，即 plugin.json 指向的可執行檔
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Python]
mydns/
├── plugin.json      # 清單檔案
├── server/
│   └── main.py      # 只使用標準函式庫的外掛
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Node.js]
mydns/
├── plugin.json      # 清單檔案
├── server/
│   └── main.js      # 沒有相依套件的外掛
├── README.md
├── LICENSE
└── CHANGELOG.md
```

:::

每個外掛套件都必須包含 `README.md`、`LICENSE` 和 `CHANGELOG.md`，因此範本已經包含了它們。發佈前請補齊內容。

## 清單檔案 {#the-manifest}

`plugin.json` 告訴 Nginx UI 這個外掛是什麼、提供什麼：

```json [plugin.json]
{
  "id": "io.github.example.mydns",
  "name": "MyDNS",
  "version": "0.1.0",
  "api_version": 1,
  "server": {
    "executables": { "linux-amd64": "dist/linux-amd64/mydns" },
    "lifecycle": "on_demand",
    "idle_timeout_seconds": 300
  },
  "capabilities": ["dns01"],
  "permissions": ["network"],
  "dns01": {
    "providers": [
      {
        "name": "MyDNS",
        "code": "mydns",
        "form": {
          "fields": [
            { "key": "MYDNS_API_TOKEN", "label": "API token", "group": "credential", "secret": true }
          ]
        }
      }
    ]
  }
}
```

- `server.executables` 把每個平台對應到為它建置的可執行檔。
- `lifecycle: "on_demand"` 表示只在憑證需要時才啟動進程，閒置五分鐘後停止。
- `permissions: ["network"]` 告訴安裝外掛的人它會存取網際網路。
- `dns01` 區塊宣告了一個服務商及其認證表單的欄位。

[清單檔案](./manifest.md)一頁介紹了每個欄位。

## 程式碼 {#the-code}

使用 Go 或 Rust SDK 時，DNS-01 外掛只需實作兩個方法：

::: code-group

```go [Go]
package main

import (
	"context"

	sdk "github.com/nginxui/plugin-sdk-go"
)

type provider struct{}

// Present publishes the challenge TXT record.
func (provider) Present(ctx context.Context, req sdk.DNS01Request) error {
	token := req.Config["MYDNS_API_TOKEN"]
	if token == "" {
		return sdk.InvalidConfig("MYDNS_API_TOKEN", "the API token is required")
	}
	// Create the TXT record req.EffectiveFQDN with the value req.Value here.
	return nil
}

// CleanUp removes what Present published.
func (provider) CleanUp(ctx context.Context, req sdk.DNS01Request) error {
	return nil
}

func main() {
	sdk.Serve(sdk.Plugin{DNS01: provider{}})
}
```

```rust [Rust]
use nginxui_plugin_sdk::{Context, Dns01Handler, Dns01Request, Error, Plugin};

struct Provider;

#[async_trait::async_trait]
impl Dns01Handler for Provider {
    // Publishes the challenge TXT record.
    async fn present(&self, _ctx: &Context, req: Dns01Request) -> Result<(), Error> {
        let token = req.config.get("MYDNS_API_TOKEN").cloned().unwrap_or_default();
        if token.is_empty() {
            return Err(Error::invalid_config("MYDNS_API_TOKEN", "the API token is required"));
        }
        // Create the TXT record req.effective_fqdn with the value req.value here.
        Ok(())
    }

    // Removes what present published.
    async fn clean_up(&self, _ctx: &Context, _req: Dns01Request) -> Result<(), Error> {
        Ok(())
    }
}

fn main() {
    nginxui_plugin_sdk::serve_blocking(Plugin::new().dns01(Provider));
}
```

:::

Go 的 `sdk.Serve` 和 Rust 的 `serve_blocking` 負責交握、健康檢查和關閉，並同時透過 gRPC 提供外掛的呼叫。[DNS-01](./capabilities/dns01.md) 一頁列出了每次呼叫攜帶的內容以及如何回報錯誤。

## 建置與檢查 {#build-and-check}

建置可執行檔，然後讓檢查工具檢查這個目錄：

```bash
cd mydns
./build.sh
nginx-ui plugin lint .
```

檢查工具會回報它發現的每個問題，並附上規則名稱，例如 `manifest-id` 或 `package-docs`。[檢查規則](./rules.md)解釋了每一條規則。

接著執行一致性測試。它會啟動外掛並測試協定和能力，但不會真正存取 DNS 服務商：

```bash
nginx-ui plugin conformance .
```

## 打包與安裝 {#package-and-install}

從目錄建立外掛套件：

```bash
nginx-ui plugin pack . io.github.example.mydns-0.1.0-linux-amd64.tar.gz
```

::: tip 提示
未簽章的外掛套件只能在開發者模式下安裝。在測試環境上開啟 **偏好設定 > 外掛**，開啟 **開發者模式**，然後開啟 **外掛** 頁面，選擇 **安裝外掛** 並上傳外掛套件。Nginx UI 會在安裝前顯示外掛請求的內容。
:::

也可以在伺服器上使用命令列：

```bash
nginx-ui plugin install io.github.example.mydns-0.1.0-linux-amd64.tar.gz --enable --approve-permissions
```

現在使用 DNS-01 驗證簽發憑證時，MyDNS 就會作為服務商出現。

## 下一步 {#next-steps}

- [開發與除錯](./development.md)：日誌、瀏覽器套件的開發載入方式，以及更快的迭代。
- [簽章與信任](./signing.md)：為外掛套件簽章，使它不需開發者模式也能安裝。
- [外掛目錄](./catalog.md)：發佈外掛，讓其他人可以從外掛市集安裝。
