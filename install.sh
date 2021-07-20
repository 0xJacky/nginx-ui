#!/bin/bash

echo "=========================="
echo
echo "Nginx UI Install Shell"
echo "Copyright (c) 0xJacky 2021"
echo
echo "=========================="

echo "Compiling api server..."
cd server || exit 1
go build -o nginx-ui-server main.go

echo "build completed"
cd ..

echo "==============="
echo "frontend dist path: nginx-ui-frontend/dist"
echo "start server, run server/nginx-ui-server"
echo "start server at background, run nohup ./nginx-ui-server &"
