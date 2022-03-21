package shield

import (
	"time"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type State struct {
	LastProcessedObjectID string
}

type AnalyticShieldData struct {
	Time    time.Time
	ID      string
	TokenID string
	Amount  string
}
