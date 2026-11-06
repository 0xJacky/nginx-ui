---
outline: [2, 3]
---

# 快速上手

本页创建一个 DNS-01 插件，构建它并安装到本地的 Nginx UI 上，大约需要十分钟。

## 准备工作 {#prerequisites}

- 一个你有管理权限的 Nginx UI，最好是测试实例。
- 编写插件所用语言的工具链：Go、Rust、Python 3 或 Node.js。
- 本机上的 `nginx-ui` 可执行文件，用于它的插件工具。

## 创建插件 {#create-the-plugin}

`nginx-ui plugin init` 会创建一个可以直接运行的插件仓库：

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

| 参数 | 含义 |
| --- | --- |
| `--id` | 插件 ID，一个反向域名，例如 `io.github.<你的 GitHub 用户名>.<插件名>`。参见[命名规则](./naming.md)。 |
| `--name` | 界面中显示的名称。 |
| `--lang` | `go`、`rust`、`python` 或 `node`。 |
| `--capability` | 起始能力。目前只支持 `dns01`。 |

每种语言都会生成一个可以直接使用的项目结构：

::: code-group

```text [Go]
mydns/
├── plugin.json      # 清单文件
├── main.go          # 基于 plugin-sdk-go 的插件
├── go.mod
├── build.sh         # 构建 dist/<平台>/mydns，即 plugin.json 指向的可执行文件
├── README.md
└── LICENSE
```

```text [Rust]
mydns/
├── plugin.json      # 清单文件
├── Cargo.toml
├── src/
│   └── main.rs      # 基于 plugin-sdk-rust 的插件
├── build.sh         # 构建 dist/<平台>/mydns，即 plugin.json 指向的可执行文件
├── README.md
└── LICENSE
```

```text [Python]
mydns/
├── plugin.json      # 清单文件
├── server/
│   └── main.py      # 只使用标准库的插件
├── README.md
└── LICENSE
```

```text [Node.js]
mydns/
├── plugin.json      # 清单文件
├── server/
│   └── main.js      # 没有依赖的插件
├── README.md
└── LICENSE
```

:::

每个插件包都必须包含 `README.md` 和 `LICENSE`，因此模板已经包含了它们。发布前请补全内容。

## 清单文件 {#the-manifest}

`plugin.json` 告诉 Nginx UI 这个插件是什么、提供什么：

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

- `server.executables` 把每个平台映射到为它构建的可执行文件。
- `lifecycle: "on_demand"` 表示只在证书需要时才启动进程，空闲五分钟后停止。
- `permissions: ["network"]` 告诉安装插件的人它会访问互联网。
- `dns01` 块声明了一个服务商及其凭据表单的字段。

[清单文件](./manifest.md)一页介绍了每个字段。

## 代码 {#the-code}

使用 Go 或 Rust SDK 时，DNS-01 插件只需实现两个方法：

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

Go 的 `sdk.Serve` 和 Rust 的 `serve_blocking` 负责握手、健康检查和关闭，并同时通过 gRPC 提供插件的调用。[DNS-01](./capabilities/dns01.md) 一页列出了每次调用携带的内容以及如何报告错误。

## 构建与检查 {#build-and-check}

构建可执行文件，然后让检查工具检查这个目录：

```bash
cd mydns
./build.sh
nginx-ui plugin lint .
```

检查工具会报告它发现的每个问题，并附上规则名称，例如 `manifest-id` 或 `package-docs`。[检查规则](./rules.md)解释了每一条规则。

接着运行一致性测试。它会启动插件并测试协议和能力，但不会真正访问 DNS 服务商：

```bash
nginx-ui plugin conformance .
```

## 打包与安装 {#package-and-install}

从目录创建插件包：

```bash
nginx-ui plugin pack . io.github.example.mydns-0.1.0-linux-amd64.tar.gz
```

::: tip 提示
未签名的插件包只能在开发者模式下安装。在测试实例上打开 **偏好设置 > 插件**，开启 **开发者模式**，然后打开 **插件** 页面，选择 **安装插件** 并上传插件包。Nginx UI 会在安装前显示插件请求的内容。
:::

也可以在服务器上使用命令行：

```bash
nginx-ui plugin install io.github.example.mydns-0.1.0-linux-amd64.tar.gz --enable --approve-permissions
```

现在使用 DNS-01 验证签发证书时，MyDNS 就会作为服务商出现。

## 下一步 {#next-steps}

- [开发与调试](./development.md)：日志、浏览器包的开发加载方式，以及更快的迭代。
- [签名与信任](./signing.md)：为插件包签名，使它无需开发者模式也能安装。
- [插件目录](./catalog.md)：发布插件，让其他人可以从插件市场安装。
