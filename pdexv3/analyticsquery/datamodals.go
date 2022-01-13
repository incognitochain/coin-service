package analyticsquery

import "encoding/json"

type PDexPairRate struct {
	High      float64 `json:"High"`
	Low       float64 `json:"Low"`
	Open      float64 `json:"Open"`
	Close     float64 `json:"Close"`
	Average   float64 `json:"Average"`
	Timestamp string `json:"Timestamp"`
}

type PDexPairRateHistoriesAPIResponse struct {
	Error  string         `json:"Error"`
	Result []PDexPairRate `json:"Result"`
}

type PDexPoolLiquidity struct {
	Token0RealAmount     float64 `json:"Token0RealAmount"`
	Token1RealAmount     float64 `json:"Token1RealAmount"`
	Token0VirtualAmount  float64 `json:"Token0VirtualAmount"`
	Token1VirtualAmount  float64 `json:"Token1VirtualAmount"`
	ShareAmount          float64 `json:"ShareAmount"`
	RateChangePercentage float64 `json:"RateChangePercentage"`
	TradingVolume24h     float64 `json:"TradingVolume24h"`
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
	Value float64 `json:"Value"`
}

type PDexSummaryDataAPIResponse struct {
	Error  string          `json:"Error"`
	Result PDexSummaryData `json:"Result"`
}
