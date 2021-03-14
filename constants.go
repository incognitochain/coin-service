package main

import "time"

const (
	MAX_COINS_INSERT_PER_REQUEST int           = 100000
	MAX_CONCURRENT_OTA_CHECK     int           = 10
	DB_OPERATION_TIMEOUT         time.Duration = 1 * time.Second
)

const (
	version             = "0.5.5"
	DefaultAPIAddress   = "0.0.0.0:9001"
	DefaultMongoAddress = "mongodb://root:example@51.161.119.66:27017"
	DefaultChainFolder  = "chain"
	DefaultMode         = INDEXERMODE
	INDEXERMODE         = "indexer"
	QUERYMODE           = "query"
)
