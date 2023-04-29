# Getting Started

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

Nginx UI is available on the following platforms:

- Mac OS X 10.10 Yosemite and later (amd64 / arm64)
- Linux 2.6.23 and later (x86 / amd64 / arm64 / armv5 / armv6 / armv7)
    - Including but not limited to Debian 7 / 8, Ubuntu 12.04 / 14.04 and later, CentOS 6 / 7, Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

You can visit [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) to download the latest distribution,
or just use [installation scripts for Linux](#script-for-linux).

## Usage

In the first runtime of Nginx UI, please visit `http://<your_server_ip>:<listen_port>/install`
in your browser to complete the follow-up configurations.

### From Executable

**Run Nginx UI in Terminal**

```shell
nginx-ui -config app.ini
```

Press `Control+C` in the terminal to exit Nginx UI.

**Run Nginx UI in Background**

```shell
nohup ./nginx-ui -config app.ini &
```

Stop Nginx UI with the follow commond.

```shell
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

### With Systemd

If you are using the [installation script for Linux](#script-for-linux), the Nginx UI will be installed as `nginx-ui`
service in systemd. Please use the `systemctl` command to control it.

**Start Nginx UI**

```shell
systemctl start nginx-ui
```

**Stop Nginx UI**

```shell
systemctl stop nginx-ui
```

**Restart Nginx UI**

```shell
systemctl restart nginx-ui
```

### With Docker

Our docker image [uozi/nginx-ui:latest](https://hub.docker.com/r/uozi/nginx-ui) is based on the latest nginx image and
can be used to replace the Nginx on the host. By publishing the container's port 80 and 443 to the host,
you can easily make the switch.

#### Note

1. When using this container for the first time, ensure that the volume mapped to /etc/nginx is empty.
2. If you want to host static files, you can map directories to container.

**Docker Deploy Example**

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

## Manual Build

On platforms that do not have an official build version, they can be built manually.

### Prerequisites

- Make

- Golang 1.19+

- node.js 18+

  ```shell
  npx browserslist@latest --update-db
  ```

### Build Frontend

Please execute the following command in `frontend` directory.

```shell
yarn install
yarn build
```

### Build Backend

Please build the frontend first, and then execute the following command in the project root directory.

```shell
go build -o nginx-ui -v main.go
```

## Script for Linux

### Basic Usage

**Install and Upgrade**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) install
```

The default listening port is `9000`, and the default HTTP Challenge port is `9180`.
If there is a port conflict, please modify `/usr/local/etc/nginx-ui/app.ini` manually,
then use `systemctl restart nginx-ui` to reload the Nginx UI service.

**Remove Nginx UI, except configuration and database files**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

### More Usage

````shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
````
