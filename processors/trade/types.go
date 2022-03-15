package trade

import (
	"time"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type State struct {
	LastProcessedObjectID string
}

type AnalyticTradeData struct {
	Time         time.Time
	TradeId      string
	Rate         float64
	PairID       string
	SellPoolID   string
	BuyPoolID    string
	Token1Amount uint64
	Token2Amount uint64
}

type tradeInfo struct {
	TokenSell  string
	TokenBuy   string
	SellAmount uint64
	BuyAmount  uint64
	Rate       float64
	TradePath  []string
	PairID     string
	IsSwap     bool
	IsBuy      bool
}
