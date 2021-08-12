#!/bin/sh
MONGOHOST1="$(getent hosts mongo1 | awk '{ print $1 }')"
# MONGOHOST2="$(getent hosts mongo2 | awk '{ print $1 }')"
# MONGOHOST3="$(getent hosts mongo3 | awk '{ print $1 }')"
INDEXERADDR="$(getent hosts indexer | awk '{ print $1 }')"
JSON='{"apiport":'"$PORT"',"chaindata":"chain","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://'"$MONGOHOST1"':27017","mongodb":"coin","chainnetwork":"'"$CHAINNETWORK"'","indexerid": '"$INDEXERID"',"masterindexer":"'"$INDEXERADDR"':9009"}'
echo $JSON 
echo $JSON > cfg.json
/home/coinservice