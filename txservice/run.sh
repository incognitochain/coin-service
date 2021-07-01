#!/bin/bash
export GOOGLE_CLOUD_PROJECT="PROJECTID"
export GOOGLE_CLOUD_ACC="PATH_TO_FILE"
export TX_TOPIC="txd"
export TXSTATUS_TOPIC="txstatus"
export TX_SUBID="txsub"
export TXSTATUS_SUBID="txstatussub"
export MONGO="mongodb://root:example@0.0.0.0:27019"
export DBNAME="txservice"
export FULLNODE="http://51.161.119.66:9334"
export SHARDNUMBER="2"


if [ "$1" == "push" ]; then
export MODE="push"
export PORT="8001"
fi
if [ "$1" == "broadcast" ]; then
export MODE="broadcast"
export PORT="8002"
fi
if [ "$1" == "status" ]; then
export MODE="status"
export PORT="8003"
fi

./txservice