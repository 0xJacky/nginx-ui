FROM --platform=linux/arm64/v8 ubuntu:latest

WORKDIR /app
EXPOSE 80 443

# COPY resources/development/sources.list /etc/apt/sources.list

ENV GO_VERSION="1.22.0"
ENV GO_ARCH="linux-arm64"
ENV GO_TAR="go${GO_VERSION}.${GO_ARCH}.tar.gz"
ENV PATH="${PATH}:/usr/local/go/bin"

RUN set -x \
    # create nginx user/group first, to be consistent throughout docker variants
    && addgroup --system --gid 101 nginx \
    && adduser --system --disabled-login --ingroup nginx --no-create-home --home /nonexistent --gecos "nginx user" --shell /bin/false --uid 101 nginx \
    && apt update && apt install gcc curl gnupg2 ca-certificates lsb-release ubuntu-keyring wget -y \
    && curl https://nginx.org/keys/nginx_signing.key | gpg --dearmor \
           | tee /usr/share/keyrings/nginx-archive-keyring.gpg >/dev/null \
    && echo "deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] \
       https://nginx.org/packages/mainline/ubuntu `lsb_release -cs` nginx" \
           | tee /etc/apt/sources.list.d/nginx.list

RUN echo "Package: *\nPin: origin nginx.org\nPin: release o=nginx\nPin-Priority: 900\n" | tee /etc/apt/preferences.d/99nginx \
    && apt update && apt install nginx -y

RUN wget https://go.dev/dl/${GO_TAR} && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf ${GO_TAR} && rm -f ${GO_TAR}

RUN go install github.com/cosmtrek/air@latest

COPY ../resources/development/entrypoint.sh /entrypoint.sh

RUN chmod a+x /entrypoint.sh  \
    && rm -f /etc/nginx/conf.d/default.conf  \
    && rm -f /usr/etc/nginx/conf.d/default.conf

CMD ["/entrypoint.sh"]
