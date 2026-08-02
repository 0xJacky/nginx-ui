# Demo image for the public instance on Cloudflare Containers.
#
# Deliberately does NOT use s6-overlay, unlike the production Dockerfile.
# s6's preinit chowns /run and its suexec calls setgid; Cloudflare Containers
# grant neither CAP_CHOWN nor CAP_SETGID, so s6 exits 111 before anything
# starts. resources/demo/entrypoint.sh starts the same processes directly.
#
# Build the binary first (see cloudflare/build-binary.sh):
#   GOWORK=off CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath \
#     -tags=jsoniter -ldflags "-s -w" -o nginx-ui-linux-amd64/nginx-ui main.go
ARG NGINX_VERSION=latest
FROM nginx:${NGINX_VERSION}
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
WORKDIR /app
# 8080 rather than 80: this image runs unprivileged (see USER below) and cannot
# bind a port under 1024.
EXPOSE 8080

ENV DEBIAN_FRONTEND=noninteractive
ENV NGINX_UI_WORKING_DIR=/var/run/nginx-ui
# Deliberately NOT setting NGINX_UI_OFFICIAL_DOCKER. It would enable a Docker
# socket self-check that can only fail here (no socket is mounted), run OTA
# container cleanup that logs errors on every boot, and default RestartCmd to
# `nginx -s stop` — which assumes an s6 supervisor that this image no longer
# has. resources/demo/app.ini sets RestartCmd explicitly instead, and
# entrypoint.sh does the supervising.

RUN apt-get update -y \
    && apt-get install -y --no-install-recommends logrotate \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /usr/local/etc \
    && mkdir /etc/nginx/sites-available \
    && mkdir /etc/nginx/sites-enabled \
    && mkdir /etc/nginx/streams-available \
    && mkdir /etc/nginx/streams-enabled \
    && cp -r /etc/nginx /usr/local/etc/nginx

# copy demo config
COPY resources/demo/ojbk.me /etc/nginx/sites-available/ojbk.me
COPY ["resources/demo/Prime Sponsor", "/etc/nginx/sites-available/Prime Sponsor"]
RUN ln -s /etc/nginx/sites-available/ojbk.me /etc/nginx/sites-enabled/ojbk.me
RUN ln -s "/etc/nginx/sites-available/Prime Sponsor" \
          "/etc/nginx/sites-enabled/Prime Sponsor"
COPY resources/demo/app.ini /etc/nginx-ui/app.ini
COPY resources/demo/demo.db /etc/nginx-ui/database.db

# copy nginx config
# The demo uses its own nginx.conf / nginx-ui.conf rather than the ones under
# resources/docker: no `user` directive, temp paths under /tmp, and port 8080,
# all so the container can run unprivileged. resources/docker/* stays as-is for
# the production image.
COPY resources/demo/nginx.conf /etc/nginx/nginx.conf
COPY resources/demo/nginx-ui.conf /etc/nginx/conf.d/nginx-ui.conf
COPY resources/demo/stub_status_nginx-ui.conf /etc/nginx/conf.d/stub_status_nginx-ui.conf
COPY resources/docker/nginx-ui.conf.known-hashes /usr/local/share/nginx-ui/nginx-ui.conf.known-hashes

# copy nginx-ui executable binary
COPY nginx-ui-$TARGETOS-$TARGETARCH$TARGETVARIANT/nginx-ui /usr/local/bin/nginx-ui

RUN rm -f /etc/nginx/conf.d/default.conf

# recreate access.log and error.log
RUN rm -f /var/log/nginx/access.log && \
    touch /var/log/nginx/access.log && \
    rm -f /var/log/nginx/error.log && \
    touch /var/log/nginx/error.log

# extra nginx-ui instances so the cluster view has real peers to talk to,
# reachable over loopback instead of a second container
COPY resources/demo/setup-cluster-nodes.sh /usr/local/bin/setup-cluster-nodes.sh
RUN chmod +x /usr/local/bin/setup-cluster-nodes.sh && \
    /usr/local/bin/setup-cluster-nodes.sh

COPY resources/demo/entrypoint.sh /usr/local/bin/demo-entrypoint.sh
RUN chmod +x /usr/local/bin/demo-entrypoint.sh

# Run unprivileged. Cloudflare Containers grant no privileged capabilities, and
# nothing here needs them once the writable paths are owned by the runtime user.
# Everything stays owned by root, and there is no USER directive. Both are
# forced by how Cloudflare Containers run this image:
#
#   1. With `USER nginx` the instance never got scheduled at all — it sat
#      'inactive' indefinitely, while an otherwise identical root image started
#      normally.
#   2. Running as root does NOT mean unrestricted: every capability is dropped,
#      so without CAP_DAC_OVERRIDE root obeys ordinary file permissions.
#      Chowning these paths to `nginx` locked root out of its own filesystem and
#      broke nginx's error log and nginx-ui's handover socket.
#
# So: root-owned, root-run, and no capabilities. The isolation comes from the
# capability set and the per-container VM, not from the uid.
RUN mkdir -p /var/cache/nginx /var/lib/nginx /var/run/nginx-ui /etc/nginx-ui

ENTRYPOINT ["/usr/local/bin/demo-entrypoint.sh"]
