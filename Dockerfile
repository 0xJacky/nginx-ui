# CGO_ENABLED=1 GOOS=linux CC=x86_64-unknown-linux-gnu-gcc CXX=x86_64-unknown-linux-gnu-g++ GOARCH=amd64 go build -ldflags "-X 'github.com/0xJacky/Nginx-UI/server/settings.buildTime=$(date +%s)'" -o nginx-ui-server -v main.go
FROM --platform=linux/amd64 uozi/nginx-ui-demo-debian-base-slim:latest
WORKDIR /app
EXPOSE 80
COPY ./resources/demo/nginx.conf /etc/nginx/sites-available/default
COPY ./resources/demo/app.ini /app/app.ini
COPY ./resources/demo/demo.db /app/database.db
COPY ./resources/demo/start.sh /app/start.sh
COPY ./nginx-ui-server /app/nginx-ui
RUN cd /app && chmod a+x start.sh
CMD ["./start.sh"]
