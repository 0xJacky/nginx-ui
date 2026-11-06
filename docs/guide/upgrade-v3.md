# Upgrading to v3

Upgrade as usual, with the installation method you used. Nginx UI v3 migrates the rest on its first start. This page lists what changes on disk and what to check on a node without internet access.

## DNS-01 Certificates

The DNS providers for DNS-01 certificates moved out of Nginx UI into the official plugin `com.nginxui.dns01`. As a result the Nginx UI binary is smaller and uses less memory when idle.

Nothing has to be done. When a node has DNS-01 certificates, Nginx UI installs and enables the plugin at startup, and checks again once a day. It looks for the package in this order:

1. the offline package directory, `<config dir>/plugins/packages`;
2. the packages bundled with the distribution. The official Docker image bundles the plugin and installs it on the first start of a container, whether or not it has DNS-01 certificates;
3. the plugin marketplace.

Certificates that waited for the plugin are renewed as soon as it is installed. If it cannot be installed, Nginx UI shows a notification, and DNS-01 certificates are not renewed until the plugin is there.

Your DNS credentials are kept. A value of [RecursiveNameservers](./config-cert.md#recursivenameservers) is copied into the plugin settings when the plugin is first installed.

### Nodes without internet access

If the node runs the official Docker image, the bundled plugin is enough. Otherwise download the package on a machine that has internet access, with Nginx UI v3 installed:

```shell
nginx-ui plugin fetch com.nginxui.dns01 --platform linux-amd64
```

Use the platform of the target node in the form `<os>-<arch>`, such as `linux-arm64`. Copy the downloaded `.tar.gz` file into `<config dir>/plugins/packages` on the node, then start or restart Nginx UI. The package can also be uploaded with **Install from file** on **System > Plugins**.

## Log Index

Log analytics is still built into Nginx UI. The index moved from Bleve to a column store, which is much smaller on disk. If indexing is enabled, on the first start of v3 Nginx UI removes the old index directories from the index directory (`log-index` next to `app.ini` by default) and indexes every log again into its `logstore` subdirectory. Search, the dashboard and the maps fill up while the rebuild runs. See [Nginx Log](./config-nginx-log.md).

## Plugin Directory

Installed plugins are kept in the plugin directory, by default `plugins` next to `app.ini`:

| Installation | Plugin directory |
| --- | --- |
| Docker | `/etc/nginx-ui/plugins`, inside the `/etc/nginx-ui` volume |
| Linux installation script | `/usr/local/etc/nginx-ui/plugins` |
| Others | `plugins` in the directory that holds `app.ini` |

Keep this directory with the rest of the configuration directory. Plugin options are described in [Plugin](./config-plugin.md).
