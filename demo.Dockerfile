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

# register nginx-ui service
COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui

# copy nginx config
COPY resources/docker/nginx.conf /usr/local/etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /usr/local/etc/nginx/conf.d/nginx-ui.conf

# copy nginx-ui executable binary
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf
