# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
FROM --platform=$TARGETPLATFORM uozi/nginx-ui-base:latest
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /app
EXPOSE 80

COPY resources/demo/ojbk.me /etc/nginx/sites-available/ojbk.me
COPY resources/demo/app.ini /etc/nginx-ui/app.ini
COPY resources/demo/demo.db /etc/nginx-ui/database.db
COPY resources/docker/nginx.conf /etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf
COPY resources/docker/start.sh /app/start.sh
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /app/nginx-ui

RUN cd /app && chmod a+x start.sh && rm -f /etc/nginx/conf.d/default.conf

ENTRYPOINT ["./start.sh"]
