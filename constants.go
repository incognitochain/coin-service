package main

import "time"

const (
	MAX_CONCURRENT_COIN_DECRYPT int           = 100
	DB_OPERATION_TIMEOUT        time.Duration = 1 * time.Second
)

const (
	version                      = "0.9.2"
	DefaultAPIPort               = 9001
	DefaultMongoAddress          = ""
	DefaultMaxConcurrentOTACheck = 10
	DefaultChainFolder           = "chain"
	DefaultFullnode              = ""
	DefaultMode                  = TESTMODE
	CHAINSYNCMODE                = "chainsync"
	INDEXERMODE                  = "indexer"
	QUERYMODE                    = "query"
	TESTMODE                     = "test"
)
