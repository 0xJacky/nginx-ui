<div align="center">
      <img src="resources/logo.png" alt="Nginx UI Logo">
</div>

# Nginx UI

Yet another Nginx Web UI, developed by [0xJacky](https://jackyu.cn/) and [Hintay](https://blog.kugeek.com/).

[![Build and Publish](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml/badge.svg)](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml)

[简体中文](README-zh_CN.md)

[繁體中文](README-zh_TW.md)

<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#demo">Demo</a></li>
        <li><a href="#features">Features</a></li>
        <li><a href="#internationalization">Internationalization</a></li>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#before-use">Before Use</a></li>
        <li><a href="#installation">Installation</a></li>
        <li>
          <a href="#usage">Usage</a>
          <ul>
            <li><a href="#from-executable">From Executable</a></li>
            <li><a href="#with-systemd">With Systemd</a></li>
            <li><a href="#with-docker">With Docker</a></li>
          </ul>
        </li>
      </ul>
    </li>
    <li>
      <a href="#manual-build">Manual Build</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#build-frontend">Build Frontend</a></li>
        <li><a href="#build-backend">Build Backend</a></li>
      </ul>
    </li>
    <li>
      <a href="#script-for-linux">Script for Linux</a>
      <ul>
        <li><a href="#basic-usage">Basic Usage</a></li>
        <li><a href="#more-usage">More Usage</a></li>
      </ul>
    </li>
    <li><a href="#example-of-nginx-reverse-proxy-configuration">Example of Nginx Reverse Proxy Configuration</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
  </ol>
</details>


## About The Project

![Dashboard](resources/screenshots/dashboard_en.png)

### Demo
URL：[https://nginxui.jackyu.cn](https://nginxui.jackyu.cn)
- Username：admin
- Password：admin

### Features

- Online view of server CPU, Memory, Load Average, Disk Usage and other indicators.
- One-click deployment and automatic renewal Let's Encrypt certificates.
- Online editing websites configurations with our self-designed **NgxConfigEditor** which is a user-friendly block editor or **Ace Code Editor** which support highlight nginx configuration syntax.
- Online view Nginx logs
- Written in Go and Vue, distribution is a single executable binary.
- Automatically test configuration file and reload nginx after saving configuration.
- Web Terminal
- Dark Mode
- Responsive Web Design

### Internationalization

- English
- Simplified Chinese
- Traditional Chinese

We welcome translations into any language.

### Built With

- [The Go Programming Language](https://go.dev)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io)
- [Vue 3](https://v3.vuejs.org)
- [Vite](https://vitejs.dev)
- [TypeScript](https://www.typescriptlang.org/)
- [Ant Design Vue](https://antdv.com)
- [vue3-gettext](https://github.com/jshmrtn/vue3-gettext)
- [vue3-ace-editor](https://github.com/CarterLi/vue3-ace-editor)

## Getting Started

### Before Use

The Nginx UI follows the Debian web server configuration file standard. Created site configuration files will be placed in the `sites-available` folder that under the Nginx configuration folder (auto-detected). The configuration files for an enabled site will create a soft link to the `sites-enabled` folder. You may need to adjust the way the configuration files are organised.

For non-Debian (and Ubuntu) systems, you may need to change the contents of the `nginx.conf` configuration file to the Debian style as shown below.

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

For more information: [debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

### Installation

Nginx UI is available on the following platforms:

- Mac OS X 10.10 Yosemite and later (amd64 / arm64)
- Linux 2.6.23 and later (x86 / amd64 / arm64 / armv5 / armv6 / armv7)
  - Including but not limited to Debian 7 / 8, Ubuntu 12.04 / 14.04 and later, CentOS 6 / 7, Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

You can visit [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) to download the latest distribution, or just use [installation scripts for Linux](#scripts-for-linux).

### Usage

In the first runtime of Nginx UI, please visit `http://<your_server_ip>:<listen_port>/install`
in your browser to complete the follow-up configurations.

#### From Executable
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
#### With Systemd
If you are using the [installation script for Linux](#scripts-for-linux), the Nginx UI will be installed as `nginx-ui` service in systemd. Please use the `systemctl` command to control it.

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

#### With Docker

You can use our `uozi/nginx-ui:latest` [image](https://hub.docker.com/r/uozi/nginx-ui) in docker, which is base on `nginx:latest`.
You can replace the Nginx on host by publishing port 80 and 443 to host.

##### Note
1. When used for the first time, the volume mapping to `/etc/nginx` should be empty.
2. If you want to host static files, map a directory to container.

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
The default listening port is 9000, and the default HTTP Challenge port is 9180. If there is a port conflict, please modify `/usr/local/etc/nginx-ui/app.ini` manually, then use `systemctl restart nginx-ui` to reload the Nginx UI service.

**Remove Nginx UI, except configuration and database files**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

### More Usage

````shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
````

## Example of Nginx Reverse Proxy Configuration

```nginx
server {
    listen          80;
    listen          [::]:80;

    server_name     <your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen  443       ssl http2;
    listen  [::]:443  ssl http2;

    server_name         <your_server_name>;

    ssl_certificate     /path/to/ssl_cert;
    ssl_certificate_key /path/to/ssl_cert_key;

    location / {
        proxy_set_header    Host                $host;
        proxy_set_header    X-Real-IP           $remote_addr;
        proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
        proxy_set_header    X-Forwarded-Proto   $scheme;
        proxy_http_version  1.1;
        proxy_set_header    Upgrade             $http_upgrade;
        proxy_set_header    Connection          $connection_upgrade;
        proxy_pass          http://127.0.0.1:9000/;
    }
}
```

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you  make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement". Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is provided under a GNU Affero General Public License v3.0 license that can be found in the [LICENSE](LICENSE) file. By using, distributing, or contributing to this project, you agree to the terms and conditions of this license.
