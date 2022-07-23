# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/server/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
FROM --platform=linux/amd64 uozi/nginx-ui-base:latest
WORKDIR /app
EXPOSE 80 443

COPY resources/docker/start.sh /app/start.sh
COPY resources/docker/nginx.conf /usr/etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /usr/etc/nginx/conf.d/nginx-ui.conf
COPY resources/docker/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf
COPY nginx-ui /app/nginx-ui

RUN cd /app && chmod a+x /app/start.sh  && rm -f /etc/nginx/conf.d/default.conf

ENTRYPOINT ["./start.sh"]
