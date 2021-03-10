package main

import "github.com/kamva/mgm/v3"

type CoinData struct {
	mgm.DefaultModel `bson:",inline"`
	CoinIndex        uint64 `json:"coinidx" bson:"coinidx"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Coin             []byte `json:"coin" bson:"coin"`
	CoinPubkey       string `json:"coinpubkey" bson:"coinpubkey"`
	OTASecret        string `json:"otasecret" bson:"otasecret"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
}

func NewCoinData(beaconHeight, idx uint64, coin []byte, tokenID, coinPubkey, OTASecret, txHash string) *CoinData {
	return &CoinData{
		CoinIndex: idx, TokenID: tokenID, Coin: coin, CoinPubkey: coinPubkey, OTASecret: OTASecret, TxHash: txHash, BeaconHeight: beaconHeight,
	}
}

type KeyImageData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	KeyImage         []byte `json:"keyimage" bson:"keyimage"`
	CoinPubkey       string `json:"coinpubkey" bson:"coinpubkey"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
}

func NewKeyImageData(tokenID, coinPubkey, txHash string, keyimage []byte, beaconHeight uint64) *KeyImageData {
	return &KeyImageData{
		TokenID: tokenID, KeyImage: keyimage, CoinPubkey: coinPubkey, TxHash: txHash, BeaconHeight: beaconHeight,
	}
}
