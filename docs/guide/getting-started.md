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
information: [debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

## Installation

We recommend using the [installation script](./install-script-linux) for Linux users, in which case you can directly
control the host machine's Nginx. You can also [install via Docker](#install-with-docker), where our provided image
includes Nginx and can be used bundled. For advanced users, you may also visit the [latest release](https://github.com/0xJacky/nginx-ui/releases/latest)
to download the latest distribution and [run executable directly](#run-executable-directly), or [manually build it](./build).

In the first runtime of Nginx UI, please visit `http://<your_server_ip>:<listen_port>/install`
in your browser to complete the follow-up configurations.

In addition, we provide [an example](./nginx-proxy-example) of using Nginx to reverse proxy Nginx UI,
which can be used after installation is complete.


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
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

In this example, port `8080` and `8443` of the container are mapped to port `80` and `443` of the host respectively.
You need to visit `http://<your_server_ip>:8080` to access Nginx UI.

## Run Executable Directly

It is not recommended to run the Nginx UI executable directly for non-testing purposes.
We recommend configuring it as a daemon or using the [installation script](./install-script-linux) on Linux.

### Config

```shell
echo '[server]\nHttpPort = 9000' > app.ini
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
