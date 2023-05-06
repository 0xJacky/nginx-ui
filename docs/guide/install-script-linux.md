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
| `-r, --reverse-proxy` | Download through a reverse proxy server (`string`)<br/>e.g., `-r https://ghproxy.com/`                          |


### Quick Usage

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) install
```

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
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

```shell [Purge]
# Remove all the Nginx UI file, include configuration and database files
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove --purge
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
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
```

## Control Service

By this script, the Nginx UI will be installed as `nginx-ui` service in systemd.
Please use the follow `systemctl` command to control it.

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

:::
