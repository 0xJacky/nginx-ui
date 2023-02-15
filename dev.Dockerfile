FROM --platform=linux/amd64 ubuntu:latest

WORKDIR /app
EXPOSE 80 443

COPY resources/development/sources.list /etc/apt/sources.list

RUN set -x \
# create nginx user/group first, to be consistent throughout docker variants
    && addgroup --system --gid 101 nginx \
    && adduser --system --disabled-login --ingroup nginx --no-create-home --home /nonexistent --gecos "nginx user" --shell /bin/false --uid 101 nginx \
    && apt update && apt install -y wget nginx gcc curl

RUN wget https://go.dev/dl/go1.20.linux-amd64.tar.gz && \
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz && rm -f go1.20.linux-amd64.tar.gz

ENV PATH="${PATH}:/usr/local/go/bin"

RUN go install github.com/cosmtrek/air@latest

COPY resources/development/entrypoint.sh /entrypoint.sh

RUN chmod a+x /entrypoint.sh  \
    && rm -f /etc/nginx/conf.d/default.conf  \
    && rm -f /usr/etc/nginx/conf.d/default.conf

CMD ["/entrypoint.sh"]
