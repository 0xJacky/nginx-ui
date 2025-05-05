# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
FROM uozi/nginx-ui-base:latest
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /app
EXPOSE 80

ENV NGINX_UI_WORKING_DIR=/var/run/

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
