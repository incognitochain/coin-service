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

type APIGetTxsByPubkeyRequest struct {
	Pubkeys []string
	Base58  bool
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

type TxBridgeDetail struct {
	Bridge          string `json:"bridge"`
	TokenID         string `json:"tokenid"`
	Amount          uint64 `json:"amount"`
	RespondTx       string `json:"respondtx"`
	RequestTx       string `json:"requesttx"`
	ShieldType      string `json:"shieldtype"`
	IsDecentralized bool   `json:"isdecentralized"`
}

type APIGetTxTradeRespond struct {
}
type ReceivedTransactionV2 struct {
	TxDetail    *shared.TransactionDetail
	FromShardID byte `json:"FromShardID"`
}

type PdexV3EstimateTradeRespond struct {
	MaxGet uint64 `json:"MaxGet"`
	Fee    uint64 `json:"Fee"`
	Route  string `json:"Route"`
}
type PdexV3OrderBookRespond struct {
	Buy  []PdexV3OrderBookVolume
	Sell []PdexV3OrderBookVolume
}
type PdexV3OrderBookVolume struct {
	Price  uint64 `json:"Price"`
	Volume uint64 `json:"Volume"`
}
type PdexV3PoolDetail struct {
	PoolID      string `json:"PoolID"`
	Token1Value uint64 `json:"Token1Value"`
	Token2Value uint64 `json:"Token2Value"`
	Share       uint64 `json:"Share"`
	Volume      uint64 `json:"Volume"`
	DayChange   uint64 `json:"24h"`
	Price       uint64 `json:"Price"`
	AMP         uint64 `json:"AMP"`
}

type PdexV3LiquidityHistoryRespond struct {
	Time   uint64 `json:"Time"`
	Volume uint64 `json:"Volume"`
}

type TradeDataRespond struct {
	RequestTx   string   `json:"requesttx"`
	RespondTxs  []string `json:"respondtxs"`
	CancelTx    []string `json:"canceltx"`
	SellTokenID string   `json:"selltokenid"`
	BuyTokenID  string   `json:"buytokenid"`
	Status      string   `json:"status"`
	PairID      string   `json:"pairid"`
	PoolID      string   `json:"poolid"`
	Price       uint64   `json:"price"`
	Amount      uint64   `json:"amount"`
	Matched     uint64   `json:"matched"`
	Requestime  int64    `json:"requesttime"`
	NFTID       string   `json:"nftid"`
	Receiver    string   `json:"receiver"`
	Fee         uint64   `json:"fee"`
	FeeToken    string   `json:"feetoken"`
}
