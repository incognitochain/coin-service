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

var InvalidPubCoinStr = []string{
	"121TuESSJu1SMeYpWsV9FaZcVYZUrF2nQo9SdsanEDmkpM2cE8q",
	"123TDL1kqsvJoEowZ1v9WSobMzFKrZudtLyLZnVp4VN1yXN7pDo",
	"12CPq9Gt72mpd2deuekEyLUkPRcbea7MyjCHMU5D1gSSY5fVZQb",
	"12FkdZV8tohXvMiE6VdZ74MdRALBsMSacrC9u6V2h4xjDL9otY3",
	"12L5ZoQL3kqA1KGKXUnwXYy4WbQGwf4y2CtKZBQuAME1S6Bp6Qq",
	"12Lokq5gRP4dg7ar7DegSQ61m1PaYVbN1gdExFicdmbgypyw745",
	"12NdqiGGeJM2rMohMYqkX2ADQWpu9yuKhkQyo5cG6z7xGaNubLC",
	"12YRTMommHZhaV1gtJWYFVW6P9JNVDpoYzjwrdCBbAU8Vw3HPf4",
	"12Z2hDZru97HuEvJrvGLfUurioLssgru9wh25CkCot2wjXLBZHm",
	"12bWPjArz154Dq5a6A6CJoDpRWiqUDx4dbvRREm8GzebP3Rfhxb",
	"12hiqDYtJ1fWKvquTy1YCwRogtKJnw86m1rAAAdkPA65cZs59jM",
	"12jwhi5LT8wUeUquvFLaePMMCuXZ2PSgvjrrEohrGuZ9EqisLZY",
	"12oKEFCVHoTwCbLJTqFRdHFRMoY7ZCXan1nrdEm3JDjaA9kLMyf",
	"12p6GFzJp5mPKVWjF3FFFZ2j38JognmEVxjXcv8FaUvNshFxfcT",
	"12tpxigXm6ZBctKyzUR9eV7knSMFJJJa18vkQUDb8LmtWktLrr5",
	"15zzRwKPjBQU6bJbRFVD4c7NZGz9xwFCNqvsUFrGhY9iJ1ehsx",
	"16D5kY3dtkypQ7SgasXW34kJDFK5aBszu8yT57cbz1i7BRD3bL",
	"18wwa6tkJQVY9dJktmQzJEjRcAWe7LTSM9xtrTNtQfCkyj95wg",
	"1AdhfpGTh5zXEWHKoDbHmEJVVfHWscytQPpKsZj9rd62AS7bu9",
	"1CiZqAP3zwxS571rWuoNpM3RsyiszSdJ6ieuqNhdiPkgKbnCfN",
	"1DjQmT5MSGeQih3LxcXURB9yLud5EEVwxyV7CyZ1BfiYEb3FfQ",
	"1Djk51zRTjMdXEDn1dCAFEiqg7Nqff9KA3sSpimV2v4EU517ru",
	"1GUaF4kmti2sTUNNhURACsXS149GwezMegAUsGw62T2fm53xUw",
	"1JUzVsmdeU9HL2PoNiyFSDr1TMA661dksgNCqVMACeFUxzJPyV",
	"1JixhUEZGVguUnQ5W6BgPhfixsY4VeDjLzTHKwvU9Q3YDaQFRA",
	"1MDXHEWKxEgKYdeaWQydpK2dnLbr5GBR9wuznx2FoC6vF6paBz",
	"1P1Am4duG1CKbnjGsUEmMtDxjrUQwu1oCyuM88gDkmqdCun9AE",
	"1PjB9qC4xdTqjmgg4pTESBts8YCGriFYPF9y8t1hbBvLd8GUwH",
	"1QGRebtUqrwzSaar5ajVXwsNotLumWquJKgt7x4dXz68LHRePf",
	"1UW4iR6d8oLBF7ZWdqrcYHY7xRMCmyAjyyRusZFEDWFXoDhxGp",
	"1Vov4RA6uWcTCyx5b2NCvtdFVu8vZXyqSvD4gMRw3dGrfXvo2i",
	"1qmxgK5wVw6t6SxYfbGhnQvGyaLzjYUFtWGZD77o6b15KPjvvL",
	"1qvkhxjKRGU4Ji1PwcaDxx6B7dPjWAC99SkHUJifZBf9Fr9rEU",
	"1rw1HBA9vmC1B6iA6g7YQZdMN2YsjUBtWL6TECvpXv8pSPmXGZ",
	"1s9KyZ88ieekJcN4bX8u94w6A7neMCGKNbPF5dueGWCGsxBA41",
	"1sAjdrAnJPF5Zp6UC9a1E5JXCz4cQAmcj4StQryH52cJwFnv3B",
	"1u2bvrQsAJwcvfeA7nWitru7o9kmCmKTduLYpyGg6ZiTt4U7fa",
	"1ugdh3P8P1v7UcFmoYkhXz9SV6nrK9zJupymL8nzTQSxdyaDAE",
	"1vTbHBTWUwEpqdwShZNAaowaFuScWRvHdL1b7rY3P4GAFXWzij",
	"1213rXNdhgh6eXRffdUp4DswV9kSaqVaA1AfjsjHWqX4y3mq714",
	"1DGAedQ9DYmLwHKw3f2vsLr1AJ8bUd51w4WN1wEdhtHy6EafxD",
	"1pnxV5Vov33unSibwNGmfQDdKj14WW1Yqta2CW9ebH7EugKUK2",
}
