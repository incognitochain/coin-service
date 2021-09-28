package assistant

import "time"

const (
	binancePriceURL string        = "https://api.binance.com/api/v3/ticker/price?symbol="
	updateInterval  time.Duration = 15 * time.Second
)
