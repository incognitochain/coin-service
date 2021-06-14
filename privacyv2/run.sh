#!/bin/sh
MONGOHOST="$(getent hosts mongov3 | awk '{ print $1 }')"
INDEXERADDR="$(getent hosts indexer0v3 | awk '{ print $1 }')"
JSON='{"apiport":'"$PORT"',"chaindata":"chain","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://root:example@'"$MONGOHOST"':27017","mongodb":"coin","chainnetwork":"testnet","highway":"'"$HIGHWAY"'","indexerid": '"$INDEXERID"',"numberofshard":'"$NUMSHARDS"',"masterindexer":"'"$INDEXERADDR"':9009"}'
echo $JSON 
echo $JSON > cfg.json
/home/coinservice