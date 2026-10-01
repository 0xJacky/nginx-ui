---
outline: [2, 3]
---

# Develop and Debug

This page collects the tools that help while a plugin is being written: a
test instance in developer mode, the development loader for browser bundles,
the logs, the linter and the conformance tests.

## Developer Mode

::: warning
Outside developer mode Nginx UI installs only signed packages. While you
develop, turn on **Preferences > Plugins > Developer Mode** on a test
instance, never on a production one. Developer mode:

- allows installing unsigned packages, from an upload, the command line or
  the offline package directory;
- shows the development tools on the **Plugins** page.
:::

It never admits a package whose signature is broken, and it does not make an
unsigned plugin eligible for automatic updates. See
[Signing and Trust](./signing.md).

## Reinstalling Quickly

Every change to the server process means a new package. Two ways keep the
loop short:

- `nginx-ui plugin pack <dir> <package>` followed by
  `nginx-ui plugin install <package> --enable --approve-permissions` on the
  server replaces the installed version in one step.
- A package dropped into the `packages` directory inside the plugin directory
  (by default `<config dir>/plugins/packages`) is installed the next time
  Nginx UI starts, when its version is newer than the installed one. Nginx UI
  then moves it to `packages/installed`.

## Developing a Browser Bundle

A browser bundle can be loaded from a local development server instead of an
installed package, so a rebuild only needs a page reload:

1. Serve the plugin directory, including `plugin.json`, from a development
   server on the machine running the browser, for example with
   `vite build --watch` and a static file server.
2. On the **Plugins** page open the menu next to the tabs and choose the
   development plugin URL. Enter the URL of `plugin.json`, such as
   `http://localhost:5173/plugin.json`.
3. Reload the page.

Nginx UI loads the bundle, stylesheet and pages that manifest names, relative
to its URL. Only `localhost` addresses are accepted, since the page runs the
script on every visit. Clear the URL to return to the installed version.

The [Browser Bundle](./webapp.md) page covers how to build the bundle.

## Logs

::: warning
A plugin writes its own log lines to **standard error**. Nginx UI captures
them and shows them in the plugin's details on the **Plugins** page. Standard
output carries protocol messages only: anything else written there breaks the
connection. The SDKs route their logger to standard error for you.
:::

A plugin can also send structured log lines with the `host.log` method, see
[Host API](./host-api.md#logging). Never log a credential or a token, see
[Permissions and Security](./permissions.md#handling-credentials).

## Checking a Plugin

### Lint

```bash
nginx-ui plugin lint <plugin directory or package>
```

The linter reads the manifest and the files of the plugin and reports every
problem at once: manifest fields, the package layout and limits, the
signature, capability blocks, templates and translation files. Each finding
has a level (`error` or `warning`) and a rule name. [Check Rules](./rules.md)
explains every rule.

A package with errors does not install. Fix warnings too: they point at
problems the person installing the plugin will see.

### Conformance Tests

```bash
nginx-ui plugin conformance <plugin directory or package> [flags]
```

The conformance tests start the plugin with Nginx UI's own supervisor and
host API and check its behavior: the handshake, health checks, error codes,
concurrent calls, shutdown, and the calls of each capability it declares.
They never call anything that would reach a real vendor or change real data:
a DNS-01 plugin is asked to validate an empty configuration, not to publish a
record.

| Flag | Meaning |
| --- | --- |
| `--capability` | Limit the capability tests to one capability, e.g. `dns01`. |
| `--transport` | `stdio`, `grpc` or `both`. Defaults to `both` when the plugin serves gRPC, `stdio` otherwise. |
| `--timeout` | Time budget of the whole run, `90s` by default. |

A plugin that serves gRPC is tested on both transports, and the results must
not differ. A plugin without a server process gets the static checks only.

### Protocol Samples

The [plugin-spec](https://github.com/nginxui/plugin-spec) repository has a
`vectors/v1` directory with a request and response sample for every method.
Use them in the unit tests of a plugin written without an SDK. The Go SDK's
`sdk.Run` and the Rust SDK's `run` drive a plugin over an in-memory pipe for
the same purpose.

## Common Problems

| Symptom | Cause |
| --- | --- |
| The plugin fails right after it starts. | Something was written to standard output before the handshake, or the executable for this platform is missing. Check the plugin logs. |
| The handshake fails with a capability mismatch. | The capabilities the process reports differ from the manifest's `capabilities`. |
| The process is restarted every few seconds. | It does not answer `plugin.ping` while busy. Answer health checks independently of slow calls, see [Lifecycle](./lifecycle.md#health-checks). |
| A permission error on a host call. | The manifest does not request the permission, or the person has not approved it yet. |
| The browser bundle does not load. | It does not call `registerPlugin` with the manifest id, or it bundles its own copy of Vue. See [Browser Bundle](./webapp.md). |
