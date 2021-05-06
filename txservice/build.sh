#!/bin/bash

if [ "$1" == "prod" ]; then
echo "building production tx-worker..."
GIN_MODE=release go build -tags=jsoniter -v -o txservice
else
echo "building tx-worker..."
go build -tags=jsoniter -v -o txservice
fi