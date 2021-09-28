package analyticsquery

import "encoding/json"

type PDexPairRate struct {
	High      uint64 `json:"High"`
	Low       uint64 `json:"Low"`
	Open      uint64 `json:"Open"`
	Close     uint64 `json:"Close"`
	Average   uint64 `json:"Average"`
	Timestamp string `json:"Timestamp"`
}

type PDexPairRateHistoriesAPIResponse struct {
	Error  string         `json:"Error"`
	Result []PDexPairRate `json:"Result"`
}

type PDexPoolLiquidity struct {
	Token0RealAmount     uint64 `json:"Token0RealAmount"`
	Token1RealAmount     uint64 `json:"Token1RealAmount"`
	Token0VirtualAmount  uint64 `json:"Token0VirtualAmount"`
	Token1VirtualAmount  uint64 `json:"Token1VirtualAmount"`
	ShareAmount          uint64 `json:"ShareAmount"`
	RateChangePercentage uint64 `json:"RateChangePercentage"`
	TradingVolume24h     uint64 `json:"TradingVolume24h"`
	Timestamp            string `json:"Timestamp"`
}

type PDexPoolLiquidityHistoriesAPIResponse struct {
	Error  string              `json:"Error"`
	Result []PDexPoolLiquidity `json:"Result"`
}

type PDexPoolRateChangesAPIResponse struct {
	Error  string                     `json:"Error"`
	Result map[string]json.RawMessage `json:"Result"`
}

type PDexSummaryData struct {
	Value uint64 `json:"Value"`
}

type PDexSummaryDataAPIResponse struct {
	Error  string          `json:"Error"`
	Result PDexSummaryData `json:"Result"`
}
