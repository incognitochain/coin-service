package apiservice

import "github.com/incognitochain/coin-service/shared"

type APICheckKeyImagesRequest struct {
	Keyimages []string
	ShardID   int
	Base58    bool
}

type APICheckTxRequest struct {
	Txs     []string
	ShardID int
}

type APISubmitOTAkeyRequest struct {
	OTAKey       string
	ShardID      int
	BeaconHeight uint64
}

type APIGetRandomCommitmentRequest struct {
	Version int
	ShardID int
	TokenID string
	//coinV2 only
	Limit int
	//coinV1 only
	Indexes []uint64
	Base58  bool
}

type APIRespond struct {
	Result interface{}
	Error  *string
}

type APILatestTxRespond struct {
	ShardID int
	Time    int
	Height  uint64
	Hash    string
	Type    int
}

type APIGetTxsBySenderRequest struct {
	Keyimages []string
	Base58    bool
}

type TxTradeDetail struct {
	RequestTx string   `json:"requesttx"`
	RespondTx []string `json:"respondtx"`
	Status    string   `json:"status"`
	BuyToken  string   `json:"buytoken"`
	SellToken string   `json:"selltoken"`
	//from respondtx
	ReceiveAmount map[string]uint64 `json:"receive"`
	//from requesttx
	SellAmount  uint64 `json:"sell"`
	Fee         uint64 `json:"fee"`
	RequestTime int64  `json:"requesttime"`
}

type APIGetTxTradeRespond struct {
}
type ReceivedTransactionV2 struct {
	TxDetail    *shared.TransactionDetail
	FromShardID byte `json:"FromShardID"`
}
