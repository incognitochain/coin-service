package main

import "github.com/kamva/mgm/v3"

type ExtraTokenInfo struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"TokenID" bson:"tokenid"`
	Name             string `json:"Name" bson:"name"`
	Symbol           string `json:"Symbol" bson:"symbol"`
	PSymbol          string `json:"PSymbol" bson:"psymbol"`
	PDecimals        uint64 `json:"PDecimals" bson:"pdecimals"`
	Decimals         uint64 `json:"Decimals" bson:"decimals"`
	ContractID       string `json:"ContractID" bson:"contractid"`
	Status           int    `json:"Status" bson:"status"`
	Type             int    `json:"Type" bson:"type"`
	CurrencyType     int    `json:"CurrencyType" bson:"currencytype"`
	Default          bool   `json:"Default" bson:"default"`
	Verified         bool   `json:"Verified" bson:"verified"`
	UserID           int    `json:"UserID" bson:"userid"`

	PriceUsd           float64 `json:"PriceUsd" bson:"priceusd"`
	PercentChange1h    string  `json:"PercentChange1h" bson:"percentchange1h"`
	PercentChangePrv1h string  `json:"PercentChangePrv1h" bson:"percentchangeprv1h"`
	CurrentPrvPool     uint64  `json:"CurrentPrvPool" bson:"currentprvpool"`
	PricePrv           float64 `json:"priceprv" bson:"priceprv"`
	Volume24           uint64  `json:"volume24" bson:"volume24"`
	ParentID           int     `json:"ParentID" bson:"parentid"`
	OriginalSymbol     string  `json:"OriginalSymbol" bson:"originalsymbol"`
	LiquidityReward    float64 `json:"LiquidityReward" bson:"liquidityreward"`

	Network        string `json:"Network" bson:"Network"`
	ListChildToken string `json:"ListChildToken" bson:"listchildtoken"`
}
