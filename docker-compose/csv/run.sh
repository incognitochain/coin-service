#!/bin/sh
MONGOHOST="$(getent hosts mongo | awk '{ print $1 }')"
INDEXERADDR="$(getent hosts indexer | awk '{ print $1 }')"
JSON='{"apiport":'"$PORT"',"chaindata":"chain","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://root:example@'"$MONGOHOST"':27017","mongodb":"coin","chainnetwork":"'"$CHAINNETWORK"'","indexerid": '"$INDEXERID"',"masterindexer":"'"$INDEXERADDR"':9009","fullnodeendpoint": "'"$FULLNODE"'","analyticsAPIEndpoint": "'"$ANALYTICS"'","externaldecimals":"'"$EXDECIMALS"'"}'
echo $JSON
echo $JSON > cfg.json
/home/coinservice