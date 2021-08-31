echo "buil frontend"
cd frontend || exit 1
yarn build
cd .. || exit 1

echo "build server"
cd server || exit 1
GOOS=linux GOARCH=amd64 go build -o nginx-ui@linux-amd64 main.go
cd .. || exit 1

echo "build docker"
docker build -t nginx-ui .
