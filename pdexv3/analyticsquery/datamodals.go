package analyticsquery

type PDexPairRate struct {
	High      float64 `json:"High"`
	Low       float64 `json:"Low"`
	Average   float64 `json:"Average"`
	Timestamp string  `json:"Timestamp"`
}

type PDexPairRateHistoriesAPIResponse struct {
	Error  string         `json:"Error"`
	Result []PDexPairRate `json:"Result"`
}

type PDexPoolLiquidity struct {
	Token0RealAmount uint64 `json:"Token0RealAmount"`
	Token1RealAmount uint64 `json:"Token1RealAmount"`
	Timestamp        string `json:"timestamp"`
}

type PDexPoolLiquidityHistoriesAPIResponse struct {
	Error  string              `json:"Error"`
	Result []PDexPoolLiquidity `json:"Result"`
}

type PDexSummaryData struct {
	Value float64 `json:"Value"`
}

type PDexSummaryDataAPIResponse struct {
	Error  string          `json:"Error"`
	Result PDexSummaryData `json:"Result"`
}
