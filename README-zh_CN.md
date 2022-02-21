# Nginx UI

Yet another Nginx Web UI

Version: 1.2.0

[For English](README.md)

## 项目特色

1. 可在线查看服务器 CPU、内存、load average、磁盘使用率等指标
2. 可一键申请和自动续签 Let's encrypt 证书
3. 在线编辑 Nginx 配置文件，编辑器支持 Nginx 配置语法高亮
4. 使用 Go 和 Vue 开发，发行版本为单个可执行的二进制文件
5. 支持简体中文和英语

## 项目预览

### 仪表盘

![](resources/screenshots/dashboard.png)

### 用户列表

![](resources/screenshots/user-list.png)

### 域名列表

![](resources/screenshots/domain-list.png)

### 域名编辑

![](resources/screenshots/domain-edit.png)

### 配置列表

![](resources/screenshots/config-list.png)

### 配置编辑

![](resources/screenshots/config-edit.png)

## 使用前注意

Nginx UI 遵循 Nginx 的标准，创建的网站配置文件位于 Nginx 配置目录（自动检测）下的 `sites-available` 目录，
启用后的网站的配置文件将会创建一份软连接到 `sites-enabled`
目录中。因此，您可能需要提前调整配置文件的组织方式。

## 安装
### 平台支持
Nginx UI 可在以下平台中可用：
- Mac OS X 10.10 Yosemite 及之后版本（amd64 / arm64）;
- Linux 2.6.23 及之后版本（x86 / amd64 / arm64）；
  - 包括但不限于 Debian 7 / 8、Ubuntu 12.04 / 14.04 及后续版本、CentOS 6 / 7、Arch Linux；
- FreeBSD (x86 / amd64)；
- OpenBSD (x86 / amd64)；
- Dragonfly BSD (amd64)；

您可以在 [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) 中下载最新发行版本

### Linux 一键安装脚本
```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh)
```
一键安装脚本默认设置的监听端口为 `9000`，HTTP Challenge 端口默认为 `9180`，
如果出现端口冲突请进入 `/usr/local/etc/nginx-ui/app.ini` 修改，并使用 `systemctl restart nginx-ui` 重启 Nginx UI 服务。

服务启动成功后，在浏览器中访问 `http://<your_server_ip>:9000/install` 完成后续配置

### Nginx 反向代理配置示例
```
server {
    listen	80;
    listen	[::]:80;

    server_name	<your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

server {
    listen	443 ssl http2;
    listen	[::]:443 ssl http2;

    server_name	<your_server_name>;

    ssl_certificate	/path/to/ssl_cert;
    ssl_certificate_key	/path/to/ssl_cert_key;

    location / {
        proxy_set_header Host $host;
        proxy_set_header   X-Real-IP            $remote_addr;
        proxy_set_header   X-Forwarded-For      $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto    $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection upgrade;
        proxy_pass http://127.0.0.1:9000/;
    }
}
```

## 关于
Nginx UI 由 [0xJacky](https://jackyu.cn/) 和 [Hintay](https://blog.kugeek.com/) 开发
