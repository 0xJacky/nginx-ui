# Install Script

This shell script currently only supports Linux systems. If you are using another operating system,
please refer to the [quick start](./getting-started) guide for manual installation or use Docker.

## Install or Upgrade

### `install.sh install`

Install or Update Nginx UI.

### Usage

```shell
install.sh install [OPTIONS]
```

### Options

| Options               |                                                                                                                 |
|-----------------------|-----------------------------------------------------------------------------------------------------------------|
| `-l, --local <file>`  | Install Nginx UI from a local file (`string`)                                                                   |
| `-p, --proxy <url>`   | Download through a proxy server (`string`)<br/>e.g., `-p http://127.0.0.1:8118` or `-p socks5://127.0.0.1:1080` |
| `-r, --reverse-proxy` | Download through a reverse proxy server (`string`)<br/>e.g., `-r https://cloud.nginxui.com/`                          |
| `-c, --channel <channel>` | Specify the version channel (`string`)<br/>Available channels: `stable` (default), `prerelease`, `dev`

#### Version Channels

| Channel      | Description                                                                                          |
|--------------|------------------------------------------------------------------------------------------------------|
| `stable`     | Latest stable release (default) - Recommended for production use                                     |
| `prerelease` | Latest prerelease version - Contains new features that are being tested before stable release       |
| `dev`        | Latest development build from dev branch - Contains the newest features but may be unstable         |

### Quick Usage

::: code-group

```shell [Stable (Default)]
# Install the latest stable version
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install
```

```shell [Prerelease]
# Install the latest prerelease version
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install --channel prerelease
```

```shell [Development]
# Install the latest development build
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ install --channel dev
```

:::

The default listening port is `9000`, and the default HTTP Challenge port is `9180`.
If there is a port conflict, please modify `/usr/local/etc/nginx-ui/app.ini` manually,
then use `systemctl restart nginx-ui` to restart the Nginx UI service.
For more information, please check [reference for config](./config-server).


## Remove

### `install.sh remove`

Remove Nginx UI.

### Usage

```shell
install.sh remove [OPTIONS]
```

### Options

| Options   |                                                                       |
|-----------|-----------------------------------------------------------------------|
| `--purge` | Remove all the Nginx UI files, include logs, configs, etc (`boolean`) |

### Quick Usage

::: code-group

```shell [Remove]
# Remove Nginx UI, except configuration and database files
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ remove
```

```shell [Purge]
# Remove all the Nginx UI file, include configuration and database files
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ remove --purge
```

:::

## Help

### `install.sh help`

Display available options.

### Usage

```shell
install.sh help
```

### Quick Usage

```shell
bash -c "$(curl -L https://cloud.nginxui.com/install.sh)" @ help
```

## Control Service

By this script, the Nginx UI will be installed as a service. The installation script detects your system's service manager and sets up the appropriate service control mechanism.

### Systemd

If your system uses systemd, please use the following `systemctl` commands to control it:

::: code-group

```shell [Start]
systemctl start nginx-ui
```

```shell [Stop]
systemctl stop nginx-ui
```

```shell [Restart]
systemctl restart nginx-ui
```

```shell [Show Status]
systemctl status nginx-ui
```

```shell [Enable at Boot]
systemctl enable nginx-ui
```

:::

### OpenRC

If your system uses OpenRC, please use the following `rc-service` commands to control it:

::: code-group

```shell [Start]
rc-service nginx-ui start
```

```shell [Stop]
rc-service nginx-ui stop
```

```shell [Restart]
rc-service nginx-ui restart
```

```shell [Show Status]
rc-service nginx-ui status
```

```shell [Enable at Boot]
rc-update add nginx-ui default
```

:::

### Init.d

If your system uses traditional init.d scripts, please use the following commands to control it:

::: code-group

```shell [Start]
/etc/init.d/nginx-ui start
```

```shell [Stop]
/etc/init.d/nginx-ui stop
```

```shell [Restart]
/etc/init.d/nginx-ui restart
```

```shell [Show Status]
/etc/init.d/nginx-ui status
```

:::
