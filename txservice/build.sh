#!/bin/bash

go get -v github.com/incognitochain/incognito-chain@9a565726ab46729ce66553d619011acf8aa10ff9
if [ "$1" == "prod" ]; then
echo "building production tx-worker..."
GIN_MODE=release go build -tags=jsoniter -v -o txservice
else
echo "building tx-worker..."
go build -tags=jsoniter -v -o txservice
fi