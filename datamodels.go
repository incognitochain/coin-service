package main

import "github.com/kamva/mgm/v3"

type CoinData struct {
	mgm.DefaultModel `bson:",inline"`
	CoinIndex        uint64 `json:"coinidx" bson:"coinidx"`
	CoinVersion      int    `json:"version" bson:"version"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Coin             []byte `json:"coin" bson:"coin"`
	CoinPubkey       string `json:"coinpubkey" bson:"coinpubkey"`
	OTASecret        string `json:"otasecret" bson:"otasecret"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
	ShardID          int    `json:"shardid" bson:"shardid"`
}

type CoinDataV1 CoinData
type CoinDataUnfinalized CoinData

func NewCoinData(beaconHeight, idx uint64, coin []byte, tokenID, coinPubkey, OTASecret, txHash string, shardID, version int) *CoinData {
	return &CoinData{
		CoinIndex: idx, CoinVersion: version, TokenID: tokenID, Coin: coin, CoinPubkey: coinPubkey, OTASecret: OTASecret, TxHash: txHash, BeaconHeight: beaconHeight, ShardID: shardID,
	}
}

type KeyImageData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	KeyImage         string `json:"keyimage" bson:"keyimage"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
	ShardID          int    `json:"shardid" bson:"shardid"`
}

type KeyImageDataUnfinalized KeyImageData

func NewKeyImageData(tokenID, txHash, keyimage string, beaconHeight uint64, shardID int) *KeyImageData {
	return &KeyImageData{
		TokenID: tokenID, KeyImage: keyimage, TxHash: txHash, BeaconHeight: beaconHeight, ShardID: shardID,
	}
}

type CoinInfo struct {
	Start uint64
	Total uint64
	End   uint64
}
type KeyInfoData struct {
	mgm.DefaultModel `bson:",inline"`
	Pubkey           string              `json:"pubkey" bson:"pubkey"`
	OTAKey           string              `json:"otakey" bson:"otakey"`
	CoinV1StartIndex map[string]CoinInfo `json:"v1startindex" bson:"v1startindex"`
	CoinV2StartIndex map[string]CoinInfo `json:"v2startindex" bson:"v2startindex"`
}

func NewKeyInfoData(Pubkey, OTAKey string, CoinV1StartIndex, CoinV2StartIndex map[string]CoinInfo) *KeyInfoData {
	return &KeyInfoData{
		Pubkey: Pubkey, OTAKey: OTAKey, CoinV1StartIndex: CoinV1StartIndex, CoinV2StartIndex: CoinV2StartIndex,
	}
}

type CoinPendingData struct {
	mgm.DefaultModel `bson:",inline"`
	SerialNumber     []string `json:"serialnum" bson:"serialnum"`
	TxHash           string   `json:"txhash" bson:"txhash"`
}

func NewCoinPendingData(SerialNumber []string, TxHash string) *CoinPendingData {
	return &CoinPendingData{
		SerialNumber: SerialNumber, TxHash: TxHash,
	}
}

type SubmittedOTAKeyData struct {
	mgm.DefaultModel `bson:",inline"`
	OTAKey           string `json:"otakey" bson:"otakey"`
	Pubkey           string `json:"pubkey" bson:"pubkey"`
	BucketID         int    `json:"bucketid" bson:"bucketid"`
}

func NewSubmittedOTAKeyData(OTAkey, pubkey string, bucketID int) *SubmittedOTAKeyData {
	return &SubmittedOTAKeyData{
		OTAKey: OTAkey, Pubkey: pubkey, BucketID: bucketID,
	}
}
