#!/bin/bash
export GIT_COMMIT=$(git rev-list -1 HEAD)
# -ldflags "-linkmode external -extldflags -static

if [ "$1" == "prod" ]; then
echo "building production coin-service..."
GOOS=linux GOARCH=amd64 GIN_MODE=release go build -tags=jsoniter -ldflags "-X main.GitCommit=$GIT_COMMIT" -v -o coinservice
else
echo "building coin-service..."
GOOS=linux GOARCH=amd64 GIN_MODE=release go build -tags=jsoniter -ldflags "-X main.GitCommit=$GIT_COMMIT" -v -o coinservice
fi
echo "finished building coin-service"