# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -o nginx-ui-server -v main.go
FROM --platform=linux/amd64 debian:buster
WORKDIR /app
COPY ./resources/demo/sources.list /etc/apt/sources.list
RUN cd /app && apt-get update -y && apt install nginx curl -y
EXPOSE 80
COPY ./resources/demo/nginx.conf /etc/nginx/sites-available/default
COPY ./resources/demo/app.ini /app/app.ini
COPY ./resources/demo/demo.db /app/database.db
COPY ./resources/demo/install.sh /app/install.sh
COPY ./resources/demo/start.sh /app/start.sh
COPY ./nginx-ui-server /app/nginx-ui
RUN cd /app && chmod a+x start.sh
CMD ["./start.sh"]
