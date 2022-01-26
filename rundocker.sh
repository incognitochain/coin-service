#!/bin/bash

echo "getting dependencies..."
go get -v
echo "building..."
GOOS=linux GIN_MODE=release go build -tags=jsoniter -v -o coinservice 
echo "running docker"
cp ./coinservice ./deploy/csv
cd deploy
env SERVICE_PORT=$1 docker-compose up -d --build