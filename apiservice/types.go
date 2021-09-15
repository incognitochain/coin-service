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
	RequestTx string
	RespondTx []string
	Status    string
	BuyToken  string
	SellToken string
	//from respondtx
	ReceiveAmount map[string]uint64
	//from requesttx
	SellAmount  uint64
	Fee         uint64
	RequestTime int64
}

type TxBridgeDetail struct {
	Bridge          string
	TokenID         string
	Amount          uint64
	RespondTx       string
	RequestTx       string
	ShieldType      string
	IsDecentralized bool
}

type APIGetTxTradeRespond struct {
}
type ReceivedTransactionV2 struct {
	TxDetail    *shared.TransactionDetail
	FromShardID byte
}

type PdexV3EstimateTradeRespond struct {
	MaxGet uint64
	Fee    uint64
	Route  string
}
type PdexV3OrderBookRespond struct {
	Buy  []PdexV3OrderBookVolume
	Sell []PdexV3OrderBookVolume
}
type PdexV3OrderBookVolume struct {
	Price  uint64
	Volume uint64
}
type PdexV3PoolDetail struct {
	PoolID         string
	Token1ID       string
	Token2ID       string
	Token1Value    uint64
	Token2Value    uint64
	Share          uint64
	AMP            uint
	Price          uint64
	Volume         uint64
	PriceChange24h uint64
	APY            uint64
}

type PdexV3LiquidityHistoryRespond struct {
	Time   uint64
	Volume uint64
}

type PdexV3PriceHistoryRespond struct {
	Timestamp uint64
	High      uint64
	Low       uint64
	Close     uint64
	Open      uint64
}
type TradeDataRespond struct {
	RequestTx   string
	RespondTxs  []string
	WithdrawTxs map[string]TradeWithdrawInfo
	SellTokenID string
	BuyTokenID  string
	Status      string
	StatusCode  int
	PairID      string
	PoolID      string
	Price       uint64
	Amount      uint64
	Matched     uint64
	Requestime  int64
	NFTID       string
	Receiver    string
	Fee         uint64
	FeeToken    string
}

type TradeWithdrawInfo struct {
	Amount    uint64
	TokenID   string
	Status    int
	RespondTx string
}

type PdexV3WithdrawRespond struct {
	RequestTx   string
	RespondTxs  []string
	Status      int
	TokenID1    string
	TokenID2    string
	Amount1     uint64
	Amount2     uint64
	ShareAmount uint64
	Requestime  int64
}

type PdexV3WithdrawFeeRespond struct {
	RequestTx  string
	RespondTxs []string
	Status     int
	TokenID1   string
	TokenID2   string
	Amount1    uint64
	Amount2    uint64
	Requestime int64
}

type PdexV3PoolShareRespond struct {
	PoolID       string
	TokenID1     string
	TokenID2     string
	Token1Amount uint64
	Token2Amount uint64
	Token1Reward uint64
	Token2Reward uint64
	Share        uint64
	AMP          uint
}

type PdexV3ContributionData struct {
	RequestTxs       []string
	RespondTxs       []string
	PairID           string
	PoolID           string
	PairHash         string
	ContributeTokens []string
	ContributeAmount []uint64
	ReturnTokens     []string
	ReturnAmount     []uint64
	// Contributor      string
	NFTID       string
	RequestTime int64
	Status      string
}

type PdexV3PairData struct {
	PairID       string
	TokenID1     string
	TokenID2     string
	Token1Amount uint64
	Token2Amount uint64
	PoolCount    int
}
