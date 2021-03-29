# Nginx UI
Yet another Nginx Web UI

Version: 0.1

# 使用前注意
Nginx UI 遵循 nginx 的标准，创建的网站配置文件位于 nginx 配置目录（自动检测）下的 sites-available 目录，
启用后的网站的配置文件将会创建一份软连接到 sites-enabled 目录中。因此，您可能需要调整配置文件的组织方式。

# 安装
1. 克隆项目
2. 运行 install.sh
3. 添加配置文件到 nginx
```
server {
	listen	80;
	listen	[::]:80;

	server_name	<Your server name>;
  	rewrite ^(.*)$  https://$host$1 permanent;
}

server {
	listen	443 ssl http2;
	listen	[::]:443 ssl http2;

	server_name	<Your server name>;

	ssl_certificate	/path/to/ssl_cert;
  	ssl_certificate_key	/path/to/ssl_cert_key;

	root	/path/to/nginx-ui-frontend/dist;
	index	index.html;

	location /api {
		rewrite /api/(.+) /$1 break;
		proxy_pass http://127.0.0.1:9000;
	}

	location /ws/ {
		proxy_set_header Host $host;
        proxy_set_header X-Real_IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr:$remote_port;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection upgrade;
		proxy_pass http://127.0.0.1:9000/;
	}
}
```
