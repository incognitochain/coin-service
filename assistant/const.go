package assistant

import "time"

const (
	binancePriceURL string        = "https://api.binance.com/api/v3/ticker/price?symbol="
	binanceMkCapURL string        = "https://www.binance.com/exchange-api/v2/public/asset-service/product/get-products"
	updateInterval  time.Duration = 20 * time.Second
)
