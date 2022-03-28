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
	FromNow      bool
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
	SellAmount    float64
	MaxGet        float64
	Fee           uint64
	Route         []string
	TokenRoute    []string
	IsSignificant bool
	ImpactAmount  float64
	// Debug         struct {
	// 	ImpactAmount float64
	// 	Rate         float64
	// 	Rate1        float64
	// }
}

type PdexV3EstimateTradeRespondBig struct {
	FeePRV   PdexV3EstimateTradeRespond
	FeeToken PdexV3EstimateTradeRespond
}

type PdexV3OrderBookRespond struct {
	Buy  []PdexV3OrderBookVolume
	Sell []PdexV3OrderBookVolume
}
type PdexV3OrderBookVolume struct {
	average float64
	Price   float64
	Volume  uint64
}
type PdexV3PoolDetail struct {
	PoolID         string
	Token1ID       string
	Token2ID       string
	Token1Value    uint64
	Token2Value    uint64
	Virtual1Value  uint64
	Virtual2Value  uint64
	TotalShare     uint64
	AMP            uint
	Price          float64
	Volume         float64
	PriceChange24h float64
	APY            uint64
	IsVerify       bool
}

type PdexV3LiquidityHistoryRespond struct {
	Timestamp           int64
	Token0RealAmount    float64 `json:"Token0RealAmount"`
	Token1RealAmount    float64 `json:"Token1RealAmount"`
	Token0VirtualAmount float64 `json:"Token0VirtualAmount"`
	Token1VirtualAmount float64 `json:"Token1VirtualAmount"`
	ShareAmount         float64 `json:"ShareAmount"`
}

type PdexV3PriceHistoryRespond struct {
	Timestamp int64
	High      float64
	Low       float64
	Close     float64
	Open      float64
}
type TradeDataRespond struct {
	RequestTx           string
	RespondTxs          []string
	RespondTokens       []string
	RespondAmounts      []uint64
	WithdrawTxs         map[string]TradeWithdrawInfo
	SellTokenID         string
	BuyTokenID          string
	Status              string
	StatusCode          int
	PairID              string
	PoolID              string
	MinAccept           uint64
	Amount              uint64
	Matched             uint64
	Requestime          int64
	NFTID               string
	Receiver            string
	Fee                 uint64
	FeeToken            string
	IsCompleted         bool
	SellTokenBalance    uint64
	BuyTokenBalance     uint64
	SellTokenWithdrawed uint64
	BuyTokenWithdrawed  uint64
	TradingPath         []string
}

type TradeWithdrawInfo struct {
	TokenIDs   []string
	IsRejected bool
	Responds   map[string]struct {
		Amount    uint64
		Status    int
		RespondTx string
	}
}

type PdexV3WithdrawRespond struct {
	PoolID      string
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
	PoolID         string
	RequestTx      string
	RespondTxs     []string
	Status         int
	WithdrawTokens map[string]uint64
	Requestime     int64
}

type PdexV3PoolShareRespond struct {
	PoolID       string
	TokenID1     string
	TokenID2     string
	Token1Amount uint64
	Token2Amount uint64
	Rewards      map[string]uint64
	OrderRewards map[string]uint64
	Share        uint64
	AMP          uint
	TotalShare   uint64
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

type PdexV3StakingPoolHistoryData struct {
	IsStaking   bool
	RequestTx   string
	RespondTx   string
	Status      int
	TokenID     string
	NFTID       string
	Amount      uint64
	Requesttime int64
}

type PdexV3StakePoolRewardHistoryData struct {
	RequestTx    string
	RespondTxs   []string
	Status       int
	TokenID      string
	RewardTokens map[string]uint64
	NFTID        string
	Requesttime  int64
}

type PdexV3PendingOrderData struct {
	TxRequest    string
	Token1Amount uint64
	Token2Amount uint64
	Token1Remain uint64
	Token2Remain uint64
	Rate         float64
}

type PdexV3StakingPoolInfo struct {
	TokenID string
	Amount  uint64
	APY     int
}

type TokenInfo struct {
	TokenID            string
	Name               string
	Symbol             string
	Image              string
	IsPrivacy          bool
	IsBridge           bool
	ExternalID         string
	PDecimals          int
	Decimals           uint64
	ContractID         string
	Status             int
	Type               int
	CurrencyType       int
	Default            bool
	Verified           bool
	UserID             int
	ListChildToken     []TokenInfo
	PSymbol            string
	OriginalSymbol     string
	LiquidityReward    float64
	ExternalPriceUSD   float64 `json:"ExternalPriceUSD"`
	PriceUsd           float64 `json:"PriceUsd"`
	PercentChange1h    string  `json:"PercentChange1h"`
	PercentChangePrv1h string  `json:"PercentChangePrv1h"`
	PercentChange24h   string  `json:"PercentChange24h"`
	CurrentPrvPool     uint64  `json:"CurrentPrvPool"`
	PricePrv           float64 `json:"PricePrv"`
	Volume24           uint64  `json:"volume24"`
	ParentID           int     `json:"ParentID"`
	Network            string
	DefaultPoolPair    string
	DefaultPairToken   string
}

type ContributionDataV1 struct {
	ID                    string `json:"id"`
	CreatedAt             string `json:"created_at"`
	UpdateAt              string `json:"updated_at"`
	RequestTx             string `json:"requesttx"`
	RespondTx             string `json:"respondtx"`
	Status                string `json:"status"`
	PairID                string `json:"pairid"`
	TokenID               string `json:"tokenid"`
	Amount                uint64 `json:"amount"`
	ReturnAmount          uint64 `json:"returnamount"`
	ContributorAddressStr string `json:"contributor"`
	Respondblock          uint64 `json:"respondblock"`
}
type APITokenInfoRequest struct {
	TokenIDs []string
	Nocache  bool
}
