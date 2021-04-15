package main

import "time"

const (
	MAX_CONCURRENT_COIN_DECRYPT int           = 100
	DB_OPERATION_TIMEOUT        time.Duration = 1 * time.Second
)

const (
	DefaultAPIPort               = 9001
	DefaultMongoAddress          = ""
	DefaultMongoDB               = "coins"
	DefaultMaxConcurrentOTACheck = 100
	DefaultChainFolder           = "chain"
	DefaultFullnode              = ""
	DefaultMode                  = TESTMODE
	DefaultNetwork               = "testnet2"
	DefaultHighway               = "74.207.247.250:9330"
	DefaultNumOfShard            = 8
)

const (
	version         = "0.9.5"
	CHAINSYNCMODE   = "chainsync"
	INDEXERMODE     = "indexer"
	QUERYMODE       = "query"
	TESTMODE        = "test"
	MAX_BUCKET_SIZE = 1000
)

const (
	MONGO_STATUS_OK   = "connected"
	MONGO_STATUS_NOK  = "disconnected"
	HEALTH_STATUS_OK  = "healthy"
	HEALTH_STATUS_NOK = "unhealthy"
)
