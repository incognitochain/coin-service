package analyticsquery

var AnalyticsAPIPath = map[string]string{
	"PDEX_V3_POOL_LIQUIDITY_HISTORIES":                 "/api/v1/metrics/pdexv3/pool-liquidity-histories",
	"PDEX_V3_PAIR_RATE_HISTORIES":                      "/api/v1/metrics/pdexv3/pair-rate-histories",
	"PDEX_V3_TRADING_VOLUME_24H":                       "/api/v1/metrics/pdexv3/trading-volume-24h",
	"PDEX_V3_TRADING_VOLUME_AND_PAIR_RATE_CHANGES_24H": "/api/v1/metrics/pdexv3/pools-trading-volume-and-pair-rate-24h",
}

const (
	Period15m = "15m"
	Period1h  = "1h"
	Period1d  = "1d"
)

const (
	period15m_table = "trade_price_15m"
	period1h_table  = "trade_price_1h"
	period1d_table  = "trade_price_1d"
)
