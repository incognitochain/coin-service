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
	PoolID       string
	Token1Amount int
	Token2Amount int
}
