package main

import "github.com/scylladb/gocqlx/table"

var coinMetadata = table.Metadata{
	Name:    "coindata",
	Columns: []string{"coin_index", "tokenID", "coin", "coin_pubkey", "ota_secret", "tx_hash", "beacon_height"},
	PartKey: []string{"tokenID", "ota_secret", "tx_hash"},
	SortKey: []string{"coin_index", "coin_pubkey", "beacon_height"},
}

var coinTable = table.New(coinMetadata)

type CoinData struct {
	CoinIndex    uint64
	TokenID      string
	Coin         []byte
	CoinPubkey   string
	OTASecret    string
	TxHash       string
	BeaconHeight uint64
}

var keyimageMetadata = table.Metadata{
	Name:    "keyimagedata",
	Columns: []string{"tokenID", "keyimage", "coin_pubkey", "tx_hash", "beacon_height"},
	PartKey: []string{"tokenID", "tx_hash"},
	SortKey: []string{"coin_pubkey", "beacon_height"},
}

var keyimageTable = table.New(keyimageMetadata)

type KeyImageData struct {
	TokenID      string
	KeyImage     []byte
	CoinPubkey   string
	TxHash       string
	BeaconHeight uint64
}
