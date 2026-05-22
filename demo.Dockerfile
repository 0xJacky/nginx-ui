# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
ARG NGINX_VERSION=latest
FROM nginx:${NGINX_VERSION}
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG S6_OVERLAY_VERSION=3.2.1.0
WORKDIR /app
EXPOSE 80

ENV DEBIAN_FRONTEND=noninteractive
ENV NGINX_UI_WORKING_DIR=/var/run/

RUN apt-get update -y \
    && apt-get install -y --no-install-recommends wget xz-utils logrotate nginx-module-geoip \
    && rm -rf /var/lib/apt/lists/*

RUN case "${TARGETARCH}/${TARGETVARIANT}" in \
        "amd64/"*) S6_ARCH="x86_64" ;; \
        "arm64/"*) S6_ARCH="aarch64" ;; \
        "arm/v7"*) S6_ARCH="arm" ;; \
        "arm/v6"*) S6_ARCH="arm" ;; \
        "arm/v5"*) S6_ARCH="arm" ;; \
        "riscv64/"*) S6_ARCH="riscv64" ;; \
        *) echo "Unsupported arch: ${TARGETARCH}/${TARGETVARIANT}" && exit 1 ;; \
    esac && \
    wget -O /tmp/s6-overlay-noarch.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-noarch.tar.xz && \
    wget -O /tmp/s6-overlay-${S6_ARCH}.tar.xz https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz && \
    tar -C / -Jxpf /tmp/s6-overlay-${S6_ARCH}.tar.xz && \
    rm -f /tmp/s6-overlay-noarch.tar.xz /tmp/s6-overlay-${S6_ARCH}.tar.xz

# register nginx service
COPY resources/docker/nginx.run /etc/s6-overlay/s6-rc.d/nginx/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx

RUN mkdir -p /usr/local/etc \
    && mkdir /etc/nginx/sites-available \
    && mkdir /etc/nginx/sites-enabled \
    && mkdir /etc/nginx/streams-available \
    && mkdir /etc/nginx/streams-enabled \
    && cp -r /etc/nginx /usr/local/etc/nginx

# init config
COPY resources/docker/init-config.up /etc/s6-overlay/s6-rc.d/init-config/up
COPY resources/docker/init-config.sh /etc/s6-overlay/s6-rc.d/init-config/init-config.sh

RUN chmod +x /etc/s6-overlay/s6-rc.d/init-config/init-config.sh && \
    echo 'oneshot' > /etc/s6-overlay/s6-rc.d/init-config/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/init-config && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx/dependencies.d/init-config

# copy demo config
COPY resources/demo/ojbk.me /etc/nginx/sites-available/ojbk.me
COPY ["resources/demo/Prime Sponsor", "/etc/nginx/sites-available/Prime Sponsor"]
RUN ln -s /etc/nginx/sites-available/ojbk.me /etc/nginx/sites-enabled/ojbk.me
RUN ln -s "/etc/nginx/sites-available/Prime Sponsor" \
          "/etc/nginx/sites-enabled/Prime Sponsor"
COPY resources/demo/app.ini /etc/nginx-ui/app.ini
COPY resources/demo/demo.db /etc/nginx-ui/database.db

# register nginx-ui service
COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui

# copy nginx config
COPY resources/docker/nginx.conf /etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf
COPY resources/demo/stub_status_nginx-ui.conf /etc/nginx/conf.d/stub_status_nginx-ui.conf

# copy nginx-ui executable binary
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf

# recreate access.log and error.log
RUN rm -f /var/log/nginx/access.log && \
    touch /var/log/nginx/access.log && \
    rm -f /var/log/nginx/error.log && \
    touch /var/log/nginx/error.log

ENTRYPOINT ["/init"]
