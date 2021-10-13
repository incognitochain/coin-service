#!/bin/bash

if [ "$1" == "prod" ]; then
echo "building production coin-worker..."
GIN_MODE=release go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -v -o coinservice
else
echo "building coin-worker..."
go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -v -o coinservice
fi