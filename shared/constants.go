package shared

import (
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/privacy/operation"
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
	DefaultMode                  = QUERYMODE
	DefaultNetwork               = "testnet2"
	DefaultHighway               = "74.207.247.250:9330"
	DefaultNumOfShard            = 8
)

const (
	VERSION       = "1.0.0"
	CHAINSYNCMODE = "chainsync"
	INDEXERMODE   = "indexer"
	QUERYMODE     = "query"
	WORKERMODE    = "indexworker"
	// FULLMODE      = "full"
	LIQUIDITYMODE   = "liquidity"
	SHIELDMODE      = "shield"
	TRADEMODE       = "trade"
	ASTMODE         = "assistant"
	COORDINATORMODE = "coordinator"
)

const (
	MONGO_STATUS_OK   = "connected"
	MONGO_STATUS_NOK  = "disconnected"
	HEALTH_STATUS_OK  = "healthy"
	HEALTH_STATUS_NOK = "unhealthy"
)

const (
	TOKENTYPE_PIRCE_STABLE = iota
	TOKENTYPE_PIRCE_VOLATILE
	TOKENTYPE_PIRCE_UNKNOW
)

const (
	BurnCoinID = "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA"
)

var AccessTokenAssetTag string

func init() {
	AccessTokenAssetTag = operation.HashToPoint(common.PdexAccessCoinID[:]).String()
}
