FROM debian:latest

WORKDIR /app

COPY ./sources.list /etc/apt/sources.list
RUN echo "installing nginx"
RUN apt-get update -y && apt install nginx -y

COPY ./start.sh /app/start.sh
RUN chmod a+x start.sh
COPY ./server /app/server
COPY ./html /app/html
COPY ./nginx.conf /etc/nginx/sites-available/default
EXPOSE 9180

CMD ["./start.sh"]
