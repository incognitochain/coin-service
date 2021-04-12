#!/bin/bash

echo "building coin-worker..."
go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static" -v -o coinservice