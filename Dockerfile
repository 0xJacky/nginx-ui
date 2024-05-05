FROM --platform=$TARGETPLATFORM uozi/nginx-ui-base:latest
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
EXPOSE 80 443

# init config
COPY resources/docker/init-config.up /etc/s6-overlay/s6-rc.d/init-config/up
COPY resources/docker/init-config.sh /etc/s6-overlay/s6-rc.d/init-config/init-config.sh

RUN chmod +x /etc/s6-overlay/s6-rc.d/init-config/init-config.sh && \
    echo 'oneshot' > /etc/s6-overlay/s6-rc.d/init-config/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/init-config && \
    mkdir -p /etc/s6-overlay/s6-rc.d/nginx/dependencies.d && \
    touch /etc/s6-overlay/s6-rc.d/nginx/dependencies.d/init-config

# register nginx-ui service
COPY resources/docker/nginx-ui.run /etc/s6-overlay/s6-rc.d/nginx-ui/run
RUN echo 'longrun' > /etc/s6-overlay/s6-rc.d/nginx-ui/type && \
    touch /etc/s6-overlay/s6-rc.d/user/contents.d/nginx-ui

# copy nginx config
COPY resources/docker/nginx.conf /usr/etc/nginx/nginx.conf
COPY resources/docker/nginx-ui.conf /usr/etc/nginx/conf.d/nginx-ui.conf
COPY resources/docker/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf

# copy nginx-ui executable binary
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /usr/local/bin/nginx-ui

# remove default nginx config
RUN rm -f /etc/nginx/conf.d/default.conf  \
    && rm -f /usr/etc/nginx/conf.d/default.conf
