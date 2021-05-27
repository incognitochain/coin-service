#!/bin/sh
cp /home/keylist.json ./keylist.json
cp /home/keylist-v2.json ./keylist-v2.json
MONGOHOST="$(getent hosts mongov3 | awk '{ print $1 }')"
INDEXERADDR="$(getent hosts indexer0v3 | awk '{ print $1 }')"
JSON='{"apiport":'"$PORT"',"chaindata":"chain","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://root:example@'"$MONGOHOST"':27017","mongodb":"coin","chainnetwork":"testnet","highway":"139.162.55.124:9330","indexerid": '"$INDEXERID"',"numberofshard":2,"masterindexer":"'"$INDEXERADDR"':9009"}'
echo $JSON 
echo $JSON > cfg.json
/home/coinservice 