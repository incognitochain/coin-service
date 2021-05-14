#!/bin/sh
# cd home
# echo "Asdasdsad"
# BUKCETID=0
# NUMBEROFBUCKET=5
# MODE="indexer"
cp /home/keylist.json ./keylist.json
cp /home/keylist-v2.json ./keylist-v2.json
MONGOHOST="$(getent hosts mongov2 | awk '{ print $1 }')"
JSON='{"apiport":'"$PORT"',"chaindata":"chain","fullnode":"http://139.162.55.124:8334","concurrentotacheck":10,"mode":"'"$MODE"'","mongo":"mongodb://root:example@'"$MONGOHOST"':27017","mongodb":"coin","chainnetwork":"testnet","highway":"139.162.55.124:9330","bucketid": '"$BUKCETID"',"numberofshard":2}'
echo $JSON 
echo $JSON > cfg.json
/home/coinservice 