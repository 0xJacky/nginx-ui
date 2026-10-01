---
outline: [2, 3]
---

# Quick Start

This page creates a DNS-01 plugin, builds it and installs it on a local
Nginx UI. It takes about ten minutes.

## Prerequisites

- An Nginx UI you can administer, ideally a test instance.
- The toolchain of the language you write the plugin in: Go, Rust, Python 3
  or Node.js.
- The `nginx-ui` binary on your machine, for its plugin tools.

## Create the Plugin

`nginx-ui plugin init` creates a working plugin repository:

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

| Flag | Meaning |
| --- | --- |
| `--id` | The plugin id, a reversed domain such as `io.github.<your GitHub name>.<plugin>`. See [Naming](./naming.md). |
| `--name` | The name shown in the interface. |
| `--lang` | `go`, `rust`, `python` or `node`. |
| `--capability` | The capability to start from. Only `dns01` is available today. |

Each language produces a ready-made layout:

::: code-group

```text [Go]
mydns/
├── plugin.json      # the manifest
├── main.go          # the plugin, built on plugin-sdk-go
├── go.mod
├── build.sh         # builds dist/<platform>/mydns, the executable plugin.json points at
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Rust]
mydns/
├── plugin.json      # the manifest
├── Cargo.toml
├── src/
│   └── main.rs      # the plugin, built on plugin-sdk-rust
├── build.sh         # builds dist/<platform>/mydns, the executable plugin.json points at
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Python]
mydns/
├── plugin.json      # the manifest
├── server/
│   └── main.py      # the plugin, standard library only
├── README.md
├── LICENSE
└── CHANGELOG.md
```

```text [Node.js]
mydns/
├── plugin.json      # the manifest
├── server/
│   └── main.js      # the plugin, no dependencies
├── README.md
├── LICENSE
└── CHANGELOG.md
```

:::

`README.md`, `LICENSE` and `CHANGELOG.md` are required in every package, so
the template already includes them. Fill them in before you publish.

## The Manifest

`plugin.json` tells Nginx UI what the plugin is and what it provides:

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

- `server.executables` maps each platform to the executable built for it.
- `lifecycle: "on_demand"` starts the process only when a certificate needs
  it and stops it after five idle minutes.
- `permissions: ["network"]` tells the person installing the plugin that it
  talks to the internet.
- The `dns01` block declares one provider and the fields of its credential
  form.

The [Manifest](./manifest.md) page describes every field.

## The Code

With the Go or Rust SDK a DNS-01 plugin implements two methods:

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

`sdk.Serve` in Go and `serve_blocking` in Rust handle the handshake, health
checks and shutdown, and also serve the plugin over gRPC. The
[DNS-01](./capabilities/dns01.md) page lists what each call carries and how to
report errors.

## Build and Check

Build the executable, then let the linter check the directory:

```bash
cd mydns
./build.sh
nginx-ui plugin lint .
```

The linter reports every problem it finds with a rule name, such as
`manifest-id` or `package-docs`. [Check Rules](./rules.md) explains each one.

Next, run the conformance tests. They start the plugin and exercise the
protocol and its capability without contacting the DNS provider:

```bash
nginx-ui plugin conformance .
```

## Package and Install

Create a package from the directory:

```bash
nginx-ui plugin pack . io.github.example.mydns-0.1.0-linux-amd64.tar.gz
```

::: tip
An unsigned package installs only while developer mode is on. On the test
instance, open **Preferences > Plugins** and turn on **Developer Mode**, then
open **Plugins**, choose **Install plugin** and upload the package. Nginx UI
shows what the plugin asks for before it installs it.
:::

The command line on the server works too:

```bash
nginx-ui plugin install io.github.example.mydns-0.1.0-linux-amd64.tar.gz --enable --approve-permissions
```

When you issue a certificate with the DNS-01 challenge, MyDNS now appears as
a provider.

## Next Steps

- [Develop and Debug](./development.md): logs, the development loader for
  browser bundles and faster iteration.
- [Signing and Trust](./signing.md): sign your packages so they install
  without developer mode.
- [Catalogs](./catalog.md): publish the plugin so others can install it from
  the marketplace.
