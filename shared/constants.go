package shared

import (
	"time"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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
	DefaultBucketSize            = 2000
)

const (
	VERSION       = "1.0.0"
	CHAINSYNCMODE = "chainsync"
	INDEXERMODE   = "indexer"
	QUERYMODE     = "query"
	TESTMODE      = "test"
)

const (
	MONGO_STATUS_OK   = "connected"
	MONGO_STATUS_NOK  = "disconnected"
	HEALTH_STATUS_OK  = "healthy"
	HEALTH_STATUS_NOK = "unhealthy"
)
