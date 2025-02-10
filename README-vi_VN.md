<div align="center">
      <img src="resources/logo.png" alt="PrimeWaf Logo">
</div>

# PrimeWaf

Yet another Nginx Web UI, được phát triển bởi [0xJacky](https://jackyu.cn/) và [Hintay](https://blog.kugeek.com/).

[![Build and Publish](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml/badge.svg)](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml)
[![GitHub license](https://img.shields.io/github/license/0xJacky/nginx-ui?label=License&logo=github)](https://github.com/0xJacky/nginx-ui "Click to view the repo on Github")
[![Release Version](https://img.shields.io/github/release/0xJacky/nginx-ui?include_prereleases&label=Release&logo=github)](https://github.com/0xJacky/nginx-ui/releases/latest "Click to view the repo on Github")
[![GitHub Star](https://img.shields.io/github/stars/0xJacky/nginx-ui?label=Stars&logo=github)](https://github.com/0xJacky/nginx-ui "Click to view the repo on Github")
[![GitHub Fork](https://img.shields.io/github/forks/0xJacky/nginx-ui?label=Forks&logo=github)](https://github.com/0xJacky/nginx-ui "Click to view the repo on Github")
[![Repo Size](https://img.shields.io/github/repo-size/0xJacky/nginx-ui?label=Size&logo=github)](https://github.com/0xJacky/nginx-ui "Click to view the repo on Github")
[![GitHub Fork](https://img.shields.io/github/issues-closed-raw/0xJacky/nginx-ui?label=Closed%20Issue&logo=github)](https://github.com/0xJacky/nginx-ui/issue "Click to view the repo on Github")

[![Docker Stars](https://img.shields.io/docker/stars/uozi/nginx-ui?label=Stars&logo=docker)](https://hub.docker.com/r/uozi/nginx-ui "Click to view the image on Docker Hub")
[![Docker Pulls](https://img.shields.io/docker/pulls/uozi/nginx-ui?label=Pulls&logo=docker)](https://hub.docker.com/r/uozi/nginx-ui "Click to view the image on Docker Hub")
[![Image Size](https://img.shields.io/docker/image-size/uozi/nginx-ui/latest?label=Image%20Size&logo=docker)](https://hub.docker.com/r/uozi/nginx-ui "Click to view the image on Docker Hub")

## Tài liệu
Để xem tài liệu, hãy truy cập [nginxui.com](https://nginxui.com).

## Stargazers over time

[![Stargazers over time](https://starchart.cc/0xJacky/nginx-ui.svg)](https://starchart.cc/0xJacky/nginx-ui)

English | [Español](README-es.md) | [简体中文](README-zh_CN.md) | [繁體中文](README-zh_TW.md) | [Tiếng Việt](README-vi_VN.md)

<details>
  <summary>Mục lục</summary>
  <ol>
    <li>
      <a href="#about-the-project">Thông tin dự án</a>
      <ul>
        <li><a href="#demo">Demo</a></li>
        <li><a href="#features">Tính năng</a></li>
        <li><a href="#internationalization">Ngôn ngữ hiển thị</a></li>
        <li><a href="#built-with">Được xây dựng với</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Bắt đầu</a>
      <ul>
        <li><a href="#before-use">Lưu ý trước khi sử dụng</a></li>
        <li><a href="#installation">Cài đặt</a></li>
        <li>
          <a href="#usage">Cách dùng</a>
          <ul>
            <li><a href="#from-executable">Sử dụng với Executable</a></li>
            <li><a href="#with-systemd">Sử dụng với Systemd</a></li>
            <li><a href="#with-docker">Sử dụng với Docker</a></li>
          </ul>
        </li>
      </ul>
    </li>
    <li>
      <a href="#manual-build">Build từ mã nguồn</a>
      <ul>
        <li><a href="#prerequisites">Điều kiện cần</a></li>
        <li><a href="#build-app">Build Frontend</a></li>
        <li><a href="#build-backend">Build Backend</a></li>
      </ul>
    </li>
    <li>
      <a href="#script-for-linux">Script cho Linux</a>
      <ul>
        <li><a href="#basic-usage">Sử dụng cơ bản</a></li>
        <li><a href="#more-usage">Sử dụng nâng cao</a></li>
      </ul>
    </li>
    <li><a href="#example-of-nginx-reverse-proxy-configuration">Ví dụ về cấu hình Nginx Reverse Proxy</a></li>
    <li><a href="#contributing">Đóng góp</a></li>
    <li><a href="#license">Giấy phép</a></li>
  </ol>
</details>

## Về dự án

![Dashboard](resources/screenshots/dashboard_en.png)

### Demo
URL：[https://demo.nginxui.com](https://demo.nginxui.com)
- Username：admin
- Password：admin

### Tính năng

- Thống kê trực tuyến cho các chỉ số máy chủ như mức sử dụng CPU, mức sử dụng bộ nhớ, mức tải trung bình và mức sử dụng ổ đĩa.
- Chat với trợ lý ChatGPT
- Triển khai bằng một cú nhấp chuột và tự động gia hạn chứng chỉ Let's Encrypt.
- Chỉnh sửa cấu hình Nginx từ UI với **NgxConfigEditor** tự thiết kế của chúng tôi, một trình chỉnh sửa khối thân thiện với người dùng cho cấu hình nginx hoặc **Ace Code Editor** hỗ trợ làm nổi bật cú pháp cấu hình nginx.
- Xem Nginx logs
- Được viết bằng Go và Vue, và được phân phối với một tệp nhị phân thực thi duy nhất.
- Tự động kiểm tra file cấu hình và tải lại nginx sau khi lưu cấu hình.
- Web Terminal
- Dark Mode
- Responsive Web Design

### Ngôn ngữ hiển thị

- Tiếng Việt
- Tiếng Anh
- Tiếng Nga
- Tiếng Pháp
- Tiếng Tây Ban Nha
- Tiếng Trung giản thể
- Tiếng Trung phồn thể

Chúng tôi hoan nghênh bản dịch sang bất kỳ ngôn ngữ nào.

### Được xây dựng với

- [The Go Programming Language](https://go.dev)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io)
- [Vue 3](https://v3.vuejs.org)
- [Vite](https://vitejs.dev)
- [TypeScript](https://www.typescriptlang.org/)
- [Ant Design Vue](https://antdv.com)
- [vue3-gettext](https://github.com/jshmrtn/vue3-gettext)
- [vue3-ace-editor](https://github.com/CarterLi/vue3-ace-editor)
- [Gonginx](https://github.com/tufanbarisyildirim/gonginx)
- [lego](https://github.com/go-acme/lego)

## Bắt đầu

### Lưu ý trước khi sử dụng

Máy chủ của bạn sẽ cần phải cài Nginx trước khi cài đặt PrimeWaf

PrimeWaf tuân theo tiêu chuẩn tệp cấu hình máy chủ web Debian. Các tệp cấu hình trang web đã tạo sẽ được đặt trong thư mục /etc/nginx/sites-available (được phát hiện tự động). Các tệp cấu hình cho một trang web được kích hoạt sẽ tạo một symlink đến thư mục /etc/nginx/sites-enabled. Bạn có thể cần điều chỉnh cách sắp xếp các tệp cấu hình của mình.

Đối với các hệ thống không phải Debian (và Ubuntu), bạn có thể cần thay đổi nội dung của tệp cấu hình nginx.conf thành kiểu Debian như hiển thị bên dưới.

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

Để biết thêm thông tin: [debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

### Cài đặt

Giao diện người dùng Nginx có sẵn trên các nền tảng sau:

- macOS 11 Big Sur and later (amd64 / arm64)
- Linux 2.6.23 và sau đó (x86 / amd64 / arm64 / armv5 / armv6 / armv7 / mips32 / mips64 / riscv64 / loongarch64)
  - Bao gồm nhưng không giới hạn Debian 7/8, Ubuntu 12.04/14.04 trở lên, CentOS 6/7, Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

Bạn có thể truy cập [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) để tải xuống bản phân phối mới nhất hoặc sử dụng [Tập lệnh cài đặt cho Linux](#script-for-linux).

### Sử dụng

Trong lần chạy đầu tiên, vui lòng truy cập `http://<your_server_ip>:<listen_port>` bằng trình duyệt của bạn để hoàn tất các cấu hình.

#### Chạy với Executable
**Chạy giao diện người dùng Nginx trong Terminal**

```shell
nginx-ui -config app.ini
```
Bấm `Ctrl + C` vào terminal để thoát PrimeWaf.

**Chạy nền (Background)**

```shell
nohup ./nginx-ui -config app.ini &
```
Dừng PrimeWaf bằng lệnh sau.

```shell
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

#### Chạy với Systemd
Nếu bạn sử dụng [tập lệnh cài đặt cho Linux](#script-for-linux), PrimeWaf sẽ được cài đặt dưới dạng `nginx-ui` service trong systemd. Hãy sử dụng `systemctl` để điều khiển nó.

**Start PrimeWaf**

```shell
systemctl start nginx-ui
```
**Stop PrimeWaf**

```shell
systemctl stop nginx-ui
```
**Restart PrimeWaf**

```shell
systemctl restart nginx-ui
```

#### Sử dụng với Docker
Docker image của chúng tôi [uozi/nginx-ui:latest](https://hub.docker.com/r/uozi/nginx-ui) dựa trên nginx image mới nhất và có thể được sử dụng để thay thế Nginx trên máy chủ. Bằng cách xuất bản cổng 80 và 443 của container, bạn có thể dễ dàng thực hiện chuyển đổi.

##### Ghi chú
1. Khi khởi chạy container lần đầu tiên, hãy chắc chắn thư mục /etc/nginx trên máy host là rỗng.
2. Nếu bạn muốn lưu trữ các tệp tĩnh, bạn có thể mount các thư mục vào container.

<details>
<summary><b>Triển khai với Docker</b></summary>

1. [Cài đặt Docker.](https://docs.docker.com/install/)

2. Sau đó triển khai nginx-ui như thế sau:

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

3. Khi container đã hoạt động, truy cập vào trang quản trị nginx-ui theo liên kết `http://<your_server_ip>:8080/install`.
</details>

<details>
<summary><b>Triển khai với Docker-Compose</b></summary>

1. [Cài đặt Docker-Compose.](https://docs.docker.com/compose/install/)

2. Tạo tệp docker-compose.yml:

```yml
services:
    nginx-ui:
        stdin_open: true
        tty: true
        container_name: nginx-ui
        restart: always
        environment:
            - TZ=Asia/Shanghai
        volumes:
            - '/mnt/user/appdata/nginx:/etc/nginx'
            - '/mnt/user/appdata/nginx-ui:/etc/nginx-ui'
            - '/var/www:/var/www'
        ports:
            - 8080:80
            - 8443:443
        image: 'uozi/nginx-ui:latest'
```

3. Sau đó tạo container bằng lệnh:
```bash
docker compose up -d
```

4. Khi container đã hoạt động, truy cập vào trang quản trị nginx-ui theo liên kết `http://<your_server_ip>:8080/install`.

</details>

## Xây dựng thủ công

Trên các nền tảng không có phiên bản xây dựng chính thức, chúng có thể được xây dựng thủ công.

### Điều kiện cần

- Make
- Golang 1.23+
- node.js 21+

  ```shell
  npx browserslist@latest --update-db
  ```

### Build Frontend

Vui lòng thực hiện lệnh sau trong thư mục `app`.

```shell
pnpm install
pnpm build
```

### Build Backend

Vui lòng build Frontend trước, sau đó thực hiện lệnh sau trong thư mục gốc của dự án.

```shell
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```

## Tập lệnh cho Linux

### Cách sử dụng cơ bản

**Cài đặt và nâng cấp**

```shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install
```
Port mặc định để truy cập UI là `9000`, port HTTP Challenge mặc định để xác thực SSL là `9180`.
Nếu có xung đột port, vui lòng sửa đổi trong file `/usr/local/etc/nginx-ui/app.ini`,
hãy nhớ restart nginx-ui bằng lệnh `systemctl restart nginx-ui` mỗi khi bạn sửa đổi file app.ini.

**Gỡ bỏ PrimeWaf nhưng giữ lại các tệp cấu hình và cơ sở dữ liệu**

```shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove
```

**Gỡ bỏ PrimeWaf đồng thời xoá các tệp cấu hình, cơ sở dữ liệu**

```shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove --purge
```

### Trợ giúp

````shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ help
````

## Ví dụ về cấu hình Nginx Reverse Proxy

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
    listen  443       ssl;
    listen  [::]:443  ssl;
    http2   on;

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

## Đóng góp

Đóng góp là điều khiến cộng đồng nguồn mở trở thành một nơi tuyệt vời để học hỏi, truyền cảm hứng và sáng tạo. Bất kỳ đóng góp nào bạn thực hiện đều được **đánh giá cao**.

Nếu bạn có đề xuất giúp dự án tốt hơn, vui lòng phân nhánh repo và tạo pull request. Bạn cũng có thể mở một issue mới với thẻ "enhancement" để đề xuất tính năng. Đừng quên cho dự án một Star! Cảm ơn một lần nữa!

1. Fork dự án
2. Tạo Branch (`git checkout -b feature/AmazingFeature`)
3. Commit thay đổi (`git commit -m 'Add some AmazingFeature'`)
4. Đẩy code lên Branch (`git push origin feature/AmazingFeature`)
5. Mở một Pull Request

## Giấy phép

Dự án này được cung cấp theo giấy phép GNU Affero General Public License v3.0 có thể tìm thấy trong tệp [LICENSE](LICENSE). Bằng cách sử dụng, phân phối hoặc đóng góp cho dự án này, bạn đồng ý với các điều khoản và điều kiện của giấy phép này.
