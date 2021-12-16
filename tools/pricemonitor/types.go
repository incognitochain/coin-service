package main

import (
	"time"
)

type ExtraTokenInfo struct {
	TokenID      string `json:"TokenID" `
	Name         string `json:"Name" `
	Symbol       string `json:"Symbol" `
	PSymbol      string `json:"PSymbol" `
	PDecimals    uint64 `json:"PDecimals" `
	Decimals     uint64 `json:"Decimals" `
	ContractID   string `json:"ContractID" `
	Status       int    `json:"Status" `
	Type         int    `json:"Type" `
	CurrencyType int    `json:"CurrencyType" `
	Default      bool   `json:"Default" `
	Verified     bool   `json:"Verified" `
	UserID       int    `json:"UserID" `

	PriceUsd           float64 `json:"PriceUsd" `
	PercentChange24h   string  `json:"PercentChange24h" `
	PercentChange1h    string  `json:"PercentChange1h" `
	PercentChangePrv1h string  `json:"PercentChangePrv1h" `
	CurrentPrvPool     uint64  `json:"CurrentPrvPool" `
	PricePrv           float64 `json:"priceprv" `
	Volume24           uint64  `json:"volume24" `
	ParentID           int     `json:"ParentID" `
	OriginalSymbol     string  `json:"OriginalSymbol" `
	LiquidityReward    float64 `json:"LiquidityReward" `

	Network         string `json:"Network" `
	ListChildToken  string `json:"ListChildToken" `
	LastPiceUpdated time.Time
	PriceUsd24h     float64
}

type APIRespond struct {
	Result interface{}
	Error  *string
}
