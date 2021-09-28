package analyticsquery

var AnalyticsAPIPath = map[string]string{
	"PDEX_V3_POOL_LIQUIDITY_HISTORIES": "/api/v1/metrics/pdexv3/pool-liquidity-histories",
	"PDEX_V3_PAIR_RATE_HISTORIES": "/api/v1/metrics/pdexv3/pair-rate-histories",
	"PDEX_V3_PAIR_RATE_CHANGES_24H": "/api/v1/metrics/pdexv3/pair-rate-changes-24h",
	"PDEX_V3_TRADING_VOLUME_24H": "/api/v1/metrics/pdexv3/trading-volume-24h",
	"PDEX_V3_TRADING_VOLUME_AND_PAIR_RATE_CHANGES_24H": "/api/v1/metrics/pdexv3/pools-trading-volume-and-pair-rate-24h",
}
