#!/bin/bash
export GIT_COMMIT=$(git rev-list -1 HEAD)
# -ldflags "-linkmode external -extldflags -static
rm coinservice
echo "building coin-service..."
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
echo "linux-gnu..."
GOOS=linux GOARCH=amd64 GIN_MODE=release go build -tags=jsoniter -ldflags "-linkmode external -extldflags -static -X main.GitCommit=$GIT_COMMIT" -v -o coinservice
elif [[ "$OSTYPE" == "darwin"* ]]; then
echo "darwin..."
GOOS=linux GOARCH=amd64 GIN_MODE=release go build -tags=jsoniter -ldflags "-X main.GitCommit=$GIT_COMMIT" -v -o coinservice
fi

echo "finished building coin-service"