#!/bin/sh

set -x
INCOGNITO_CHAIN_VERSION=$(grep 'github.com/incognitochain/incognito-chain' go.mod | awk '{ print $2}')

echo "$INCOGNITO_CHAIN_VERSION"
INCOGNITO_CHAIN_SRC_PATH="$GOPATH/pkg/mod/github.com/incognitochain/incognito-chain@$INCOGNITO_CHAIN_VERSION"
mkdir -p /app/config
cp -r "$INCOGNITO_CHAIN_SRC_PATH/config/mainnet" /app/config/mainnet
cp -r "$INCOGNITO_CHAIN_SRC_PATH/config/testnet-1" /app/config/testnet-1
cp -r "$INCOGNITO_CHAIN_SRC_PATH/config/testnet-2" /app/config/testnet-2

echo "$INCOGNITO_CHAIN_SRC_PATH"
