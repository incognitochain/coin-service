#!/bin/sh

CONFIG_JSON=$(cat <<EOF
{
   "apiport": $API_PORT,
   "chaindata": "$CHAIN_DATA",
   "concurrentotacheck": $CONCURRENT_OTA_CHECK,
   "mode": "$MODE",
   "mongo": "$MONGO",
   "mongodb": "$MONGO_DB",
   "chainnetwork": "$CHAIN_NETWORK",
   "indexerid": $INDEXER_ID,
   "masterindexer": "$MASTER_INDEXER",
   "fullnodeendpoint": "$FULLNODE_ENDPOINT",
   "analyticsAPIEndpoint": "$ANALYTICS_API_ENDPOINT",
   "fullnodedata": $FULLNODEDATA,
   "externaldecimals": "$EXTERNAL_DECIMALS"
}
EOF
)

echo $CONFIG_JSON
printf "$CONFIG_JSON" > cfg.json

# "$@" to pass through all arguments to script
./coinservice "$@"