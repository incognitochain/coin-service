#!/bin/sh
MONGOHOST="$(getent hosts mongo | awk '{ print $1 }')"
INDEXERADDR="$(getent hosts indexer | awk '{ print $1 }')"
COORIDANTORADDR="$(getent hosts coordinator | awk '{ print $1 }')"
LOGRECORDER="$(getent hosts coordinator | awk '{ print $1 }')"

JSON='{"apiport":'"$PORT"',"chaindata":"chain","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://root:example@'"$MONGOHOST"':27017","mongodb":"coin","chainnetwork":"'"$CHAINNETWORK"'","indexerid": '"$INDEXERID"',"masterindexer":"'"$INDEXERADDR"':9009","analyticsAPIEndpoint": "'"$ANALYTICS"'","externaldecimals":"'"$EXDECIMALS"'","fullnodedata":'"$FULLNODEDATA"',"coordinator":"'"$COORIDANTORADDR"':9009","logrecorder":"'"$LOGRECORDER"':9008"}'
echo $JSON
echo $JSON > cfg.json
./coinservice