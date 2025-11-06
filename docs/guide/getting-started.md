# Getting Started

## Try It Now

You can try Nginx UI directly by the [demo](https://demo.nginxui.com).

- Username：admin
- Password：admin

## Before Use

The Nginx UI follows the Debian web server configuration file standard. Created site configuration files will be placed
in the `sites-available` folder that under the Nginx configuration folder (auto-detected). The configuration files for
an enabled site will create a soft link to the `sites-enabled` folder. You may need to adjust the way the configuration
files are organised.

For non-Debian (and Ubuntu) systems, you may need to change the contents of the `nginx.conf` configuration file to the
Debian style as shown below.

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

For more
information: [debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/debian/latest/debian/conf/nginx.conf#L60-L61)

## Installation

We provide several installation methods to suit different needs:

- **macOS/Linux**: Use [Homebrew](./install-homebrew) for the easiest installation
- **Windows**: Use [Winget](./install-winget) for Windows package management
- **Linux**: Use the [installation script](./install-script-linux) to directly control the host machine's Nginx
- **Docker**: [Install via Docker](#install-with-docker) with our bundled image that includes Nginx
- **Advanced**: Download from [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) and [run executable directly](#run-executable-directly), or [manually build it](./build)

In the first runtime of Nginx UI, please visit `http://<your_server_ip>:<listen_port>`
in your browser to complete the follow-up configurations.

In addition, we provide [an example](./nginx-proxy-example) of using Nginx to reverse proxy Nginx UI,
which can be used after installation is complete.

## Install with Homebrew

For macOS and Linux users, you can install Nginx UI using Homebrew, which provides the easiest installation experience.

::: tip

This installation method is available for macOS and Linux. For other operating systems, please use alternative installation methods.

:::

### Install

```bash
brew install 0xjacky/tools/nginx-ui
```

### Start Service

```bash
# Start the service
brew services start nginx-ui

# Or run in foreground
nginx-ui
```

### Stop Service

```bash
brew services stop nginx-ui
```

### Upgrade

```bash
brew upgrade nginx-ui
```

### Uninstall

```bash
# Stop the service first
brew services stop nginx-ui

# Uninstall the package
brew uninstall nginx-ui

# Optionally remove the tap
brew untap 0xjacky/tools
```

::: warning

After uninstalling, configuration files and data will be preserved in:
- **macOS**: `~/Library/Application Support/nginx-ui/`
- **Linux**: `~/.local/share/nginx-ui/` or `~/.config/nginx-ui/`

If you want to completely remove all data, please delete these directories manually.

:::

## Install with Docker

Our docker image [uozi/nginx-ui:latest](https://hub.docker.com/r/uozi/nginx-ui) is based on the latest nginx image and
can be used to replace the Nginx on the host. By publishing the container's port 80 and 443 to the host,
you can easily make the switch.

::: tip

Nginx UI is by default proxied to port `8080` of the container.
When using this container for the first time, ensure that the volume mapped to `/etc/nginx` is empty.
If you want to host static files, you can map directories to container.

:::

::: warning


If you want to manage the Nginx of the host, please choose another installation method.
We recommend using the [installation script](./install-script-linux) if you are using Linux.

:::

### Docker Deploy Example

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -v /var/www:/var/www \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

In this example, port `80` and `443` of the container are mapped to port `8080` and `8443` of the host respectively.
You need to visit `http://<your_server_ip>:8080` to access Nginx UI.

## Run Executable Directly

It is not recommended to run the Nginx UI executable directly for non-testing purposes.
We recommend configuring it as a daemon or using the [installation script](./install-script-linux) on Linux.

### Config

```shell
echo '[server]\nPort = 9000' > app.ini
```

::: tip

The server can still be started without `app.ini`, it will listen on the default port `9000`.

:::

### Run

::: code-group

```shell [In Terminal]
nginx-ui -config app.ini
```

```shell [In Background]
nohup ./nginx-ui -config app.ini &
```

:::


### Stop

::: code-group

```shell [In Terminal]
^C   # Press Ctrl+C
```

```shell [In Background]
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

:::
