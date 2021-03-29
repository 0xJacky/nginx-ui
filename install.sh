#!/bin/bash

echo "=========================="
echo
echo "Nginx UI Install Shell"
echo "Copyright (c) 0xJacky 2021"
echo
echo "=========================="

echo "installing yarn..."
npm install -g yarn

echo "Compiling frontend..."
cd nginx-ui-frontend || exit 1
yarn build

cd ..

echo "Compiling api server..."
cd server || exit 1
go build -o nginx-ui-server main.go

echo "Installing acme.sh..."
go test -v test/acme_test.go

echo "build completed"
cd ..

echo "==============="
echo "frontend dist path: nginx-ui-frontend/dist"
echo "start server, run server/nginx-ui-server"
