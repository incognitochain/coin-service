package assistant

import (
	"time"

	jsoniter "github.com/json-iterator/go"
)

const (
	binancePriceURL   string        = "https://api.binance.com/api/v3/ticker/price?symbol="
	coingeckoMkCapURL string        = "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&order=market_cap_desc&per_page=250&page="
	updateInterval    time.Duration = 20 * time.Second
)
const (
	defaultPoolsKey string = "dfpool"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	scanQualifyPoolsInterval = 4 * time.Hour
)
