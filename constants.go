package main

import "time"

const (
	MAX_COINS_INSERT_PER_REQUEST int = 100000
	// MAX_CONCURRENT_OTA_CHECK     int           = 10
	DB_OPERATION_TIMEOUT time.Duration = 1 * time.Second
)

const (
	version                      = "0.9.0"
	DefaultAPIPort               = 9001
	DefaultMongoAddress          = "mongodb://root:example@0.0.0.0:27017"
	DefaultMaxConcurrentOTACheck = 10
	DefaultChainFolder           = "chain"
	DefaultMode                  = INDEXERMODE
	CHAINSYNCMODE                = "chainsync"
	INDEXERMODE                  = "indexer"
	QUERYMODE                    = "query"
)
