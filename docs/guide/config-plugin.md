# Plugin

- Version: `>= v3.0.0`

Plugins add capabilities to Nginx UI, such as the DNS providers for DNS-01 certificates. They are managed on **System > Plugins**, and the options of this section are shown in **Preferences > Plugins**. Writing a plugin is covered in the [plugin guide](/plugin/overview).

## Installing Plugins

A plugin can be installed:

- from the marketplace, on the **Marketplace** tab of the Plugins page;
- from a package file (`.tar.gz`) with **Install from file**, or with `nginx-ui plugin install <package>`;
- offline, by copying the package into the `packages` directory inside the plugin directory (by default `<config dir>/plugins/packages`). Nginx UI installs it the next time it starts, when it is newer than the installed version, and then moves it to `packages/installed`.

To get a package for a node without internet access, run `nginx-ui plugin fetch <plugin id> --platform <os>-<arch>` on a machine that can reach the marketplace, for example `--platform linux-amd64`. `--platform all` downloads the packages of every platform.

Every package is signed. Its signer decides its trust level:

| Level | Signer | Installs |
| --- | --- | --- |
| `official` | the Nginx UI project | normally |
| `verified` | a partner the project vouches for | normally |
| `community` | the plugin author | when [AllowCommunityPlugins](#allowcommunityplugins) is on, after a confirmation |
| `unsigned` | nobody, or an unknown key | only in [DeveloperMode](#developermode) |

Before a plugin runs, it lists the permissions it asks for, and it stays disabled until you approve them. See [Signing and Trust](/plugin/signing) for the details.

## DNS-01 Plugin

The DNS providers used by DNS-01 certificates are not part of Nginx UI itself since v3.0.0. They are provided by the official plugin `com.nginxui.dns01`, which takes about 60 MB once installed.

When a certificate uses the DNS-01 challenge and the plugin is missing, Nginx UI installs it on its own, at startup and once a day. It takes the package from the offline `packages` directory first, then from [BundledDir](#bundleddir), then from the marketplace, and only accepts an `official` package. The official Docker image bundles the plugin and installs and enables it on the first start of a container.

After you uninstall the plugin it is not installed again, unless a certificate still needs DNS-01.

## Enabled

- Type: `boolean`
- Default: `true`

The switch of the whole plugin system. Turning it off stops every plugin and hides the plugin pages.

## Dir

- Type: `string`
- Default: `plugins` in the directory that holds `app.ini`

The directory of the installed plugins and of the `packages` directory for offline installs. In the official Docker image this is `/etc/nginx-ui/plugins`, inside the `/etc/nginx-ui` volume. This option can only be set in `app.ini` or with its environment variable.

## BundledDir

- Type: `string`
- Default: empty

A read-only directory of plugin packages that a distribution ships next to the binary. Each bundled plugin is installed and enabled once per node and upgraded when the bundled package is newer. A bundled plugin you uninstalled does not come back. Only `official` packages are installed from this directory. The official Docker image sets it to `/usr/local/share/nginx-ui/plugins`. This option can only be set in `app.ini` or with its environment variable.

## DefaultSyncPolicy

- Type: `string`
- Default: `manual`
- Values: `manual`, `auto`

The cluster sync policy given to newly installed plugins. `auto` keeps the plugin installed on the child nodes, with the same version and enabled state.

## MarketplaceEnabled

- Type: `boolean`
- Default: `true`

Turns the marketplace on. Without it, plugins install from files and from the offline directory only.

## MarketplaceSources

- Type: `[]string`
- Default: empty

URLs of additional plugin catalogs, in the order they are merged. The official catalog `https://plugins.nginxui.com/v1/index.json` is always read, in front of the others. Sources are managed on the Marketplace tab of the Plugins page.

## TrustedPublicKeys

- Type: `[]string`
- Default: empty

Minisign public keys of trusted publishers, each optionally followed by a space and the publisher name. Packages signed with these keys install as `community` plugins, also when they are uploaded or copied into the offline directory. Shown as **Trusted Publishers** in Preferences.

## AllowCommunityPlugins

- Type: `boolean`
- Default: `true`

Allows installing `community` plugins. Each install asks for a confirmation.

## AllowUploads

- Type: `boolean`
- Default: `true`

Allows installing packages uploaded from the browser or with the command line.

## AutoUpdate

- Type: `boolean`
- Default: `false`

Updates `official` and `verified` plugins on their own, as long as the permissions they ask for stay the same.

## AllowInsecureDownloadURL

- Type: `boolean`
- Default: `false`

Accepts plain `http` catalog and download addresses. Only use it for a private catalog on a trusted network.

## DeveloperMode

- Type: `boolean`
- Default: `false`

Allows installing unsigned plugins from every source. Only turn it on while developing a plugin, never on a production node.

## MemoryLimitMB

- Type: `int`
- Default: `0` (unlimited)

The memory limit of every plugin process, in MiB. A plugin that asks for a smaller limit gets its own. Applies on Linux with cgroup v2 only.

## CPUPercent

- Type: `int`
- Default: `0` (unlimited)

The CPU limit of every plugin process, in percent of one core: `100` is one core. A plugin that asks for a smaller limit gets its own. Applies on Linux with cgroup v2 only.

## CgroupRoot

- Type: `string`
- Default: `/sys/fs/cgroup`

The cgroup v2 mount point the limits are enforced under. Each plugin gets `<CgroupRoot>/nginx-ui/plugins/<id>`. This option can only be set in `app.ini` or with its environment variable.
