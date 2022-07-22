#!/bin/bash
CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc \
    CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags \
    "-X 'github.com/0xJacky/Nginx-UI/server/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go

docker build -t nginx-ui .
docker tag nginx-ui uozi/nginx-ui
docker push uozi/nginx-ui
