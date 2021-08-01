package main

import "github.com/kamva/mgm/v3"

// Read from shard
type ContributionData struct {
	mgm.DefaultModel      `bson:",inline"`
	RequestTx             string `json:"requesttx" bson:"requesttx"`
	RespondTx             string `json:"respondtx" bson:"respondtx"`
	Status                string `json:"status" bson:"status"`
	PairID                string `json:"pairid" bson:"pairid"`
	TokenID               string `json:"tokenid" bson:"tokenid"`
	Amount                uint64 `json:"amount" bson:"amount"`
	ReturnAmount          uint64 `json:"returnamount" bson:"returnamount"`
	ContributorAddressStr string `json:"contributor" bson:"contributor"`
	Respondblock          uint64 `json:"respondblock" bson:"respondblock"`
}

type WithdrawContributionData struct {
	mgm.DefaultModel      `bson:",inline"`
	RequestTx             string   `json:"requesttx" bson:"requesttx"`
	RespondTx             []string `json:"respondtx" bson:"respondtx"`
	Status                string   `json:"status" bson:"status"`
	TokenID1              string   `json:"tokenid1" bson:"tokenid1"`
	TokenID2              string   `json:"tokenid2" bson:"tokenid2"`
	Amount1               uint64   `json:"amount1" bson:"amount1"`
	Amount2               uint64   `json:"amount2" bson:"amount2"`
	ContributorAddressStr string   `json:"contributor" bson:"contributor"`
	RespondTime           int64    `json:"respondtime" bson:"respondtime"`
}

type WithdrawContributionFeeData struct {
	mgm.DefaultModel      `bson:",inline"`
	RequestTx             string `json:"requesttx" bson:"requesttx"`
	RespondTx             string `json:"respondtx" bson:"respondtx"`
	Status                string `json:"status" bson:"status"`
	TokenID1              string `json:"tokenid1" bson:"tokenid1"`
	TokenID2              string `json:"tokenid2" bson:"tokenid2"`
	Amount                uint64 `json:"amount" bson:"amount"`
	ContributorAddressStr string `json:"contributor" bson:"contributor"`
	RespondTime           int64  `json:"respondtime" bson:"respondtime"`
}

// Breaking change
type TradeData struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTx        string   `json:"requesttx" bson:"requesttx"`
	RespondTx        []string `json:"respondtx" bson:"respondtx"`
	WithdrawTx       string   `json:"withdrawtx" bson:"withdrawtx"`
	PairID           string   `json:"pairid" bson:"pairid"`
	Status           string   `json:"status" bson:"status"`
	Type             string   `json:"type" bson:"type"`
}

////////////////////////////////////
// Read from beacon pdex state
type PoolPairData struct {
	mgm.DefaultModel `bson:",inline"`
	PairID           string `json:"pairid" bson:"pairid"`
	Token1           string `json:"token1" bson:"token1"`
	Token2           string `json:"token2" bson:"token2"`
	AMP              int    `json:"amp" bson:"amp"`
	Token1Amount     string `json:"token1amount" bson:"token1amount"`
	Token2Amount     string `json:"token2amount" bson:"token2amount"`
}

type OrderData struct {
	mgm.DefaultModel `bson:",inline"`
	Requesttx        string `json:"requesttx" bson:"requesttx"`
	WithdrawTx       string `json:"withdrawtx" bson:"withdrawtx"`
	PairID           string `json:"pairid" bson:"pairid"`
	SellToken        string `json:"selltoken" bson:"selltoken"`
	SellRemain       uint64 `json:"sellremain" bson:"sellremain"`
	FilledAmount     uint64 `json:"filled" bson:"filled"`
	Price            int    `json:"price" bson:"price"`
}

// New metadata type
const (
	Pdexv3ModifyParamsMeta = 270

	// Pdexv3AddLiquidityRequestMeta       = 271
	Pdexv3AddLiquidityResponseMeta = 272

	// Pdexv3WithdrawLiquidityRequestMeta  = 273
	Pdexv3WithdrawLiquidityResponseMeta = 274

	// Pdexv3TradeRequestMeta              = 275
	PDexv3TradeResponseMeta = 276

	Pdexv3AddOrderRequestMeta  = 277
	Pdexv3AddOrderResponseMeta = 278

	Pdexv3WithdrawOrderRequestMeta  = 279
	Pdexv3WithdrawOrderResponseMeta = 280
)
