package shared

import (
	"strconv"

	"github.com/kamva/mgm/v3"
)

type CoinData struct {
	mgm.DefaultModel `bson:",inline"`
	CoinIndex        uint64 `json:"coinidx" bson:"coinidx"`
	CoinVersion      int    `json:"version" bson:"version"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	RealTokenID      string `json:"realtokenid" bson:"realtokenid"`
	Coin             []byte `json:"coin" bson:"coin"`
	CoinPubkey       string `json:"coinpubkey" bson:"coinpubkey"`
	OTASecret        string `json:"otasecret" bson:"otasecret"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
	ShardID          int    `json:"shardid" bson:"shardid"`
	IsNFT            bool   `json:"isnft" bson:"isnft"`
}

type CoinDataV1 CoinData

func NewCoinData(beaconHeight, idx uint64, coin []byte, tokenID, coinPubkey, OTASecret, txHash string, shardID, version int) *CoinData {
	return &CoinData{
		CoinIndex: idx, CoinVersion: version, TokenID: tokenID, Coin: coin, CoinPubkey: coinPubkey, OTASecret: OTASecret, TxHash: txHash, BeaconHeight: beaconHeight, ShardID: shardID,
	}
}

func (model *CoinData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *CoinData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type KeyImageData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	KeyImage         string `json:"keyimage" bson:"keyimage"`
	TxHash           string `json:"txhash" bson:"txhash"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
	ShardID          int    `json:"shardid" bson:"shardid"`
}

func NewKeyImageData(tokenID, txHash, keyimage string, beaconHeight uint64, shardID int) *KeyImageData {
	return &KeyImageData{
		TokenID: tokenID, KeyImage: keyimage, TxHash: txHash, BeaconHeight: beaconHeight, ShardID: shardID,
	}
}
func (model *KeyImageData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *KeyImageData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type CoinInfo struct {
	Start       uint64
	Total       uint64
	End         uint64
	LastScanned uint64
}
type KeyInfoData struct {
	mgm.DefaultModel `bson:",inline"`
	Pubkey           string              `json:"pubkey" bson:"pubkey"`
	OTAKey           string              `json:"otakey" bson:"otakey"`
	CoinIndex        map[string]CoinInfo `json:"coinindex" bson:"coinindex"`
	NFTIndex         map[string]CoinInfo `json:"nftindex" bson:"nftindex"`
	TotalReceiveTxs  map[string]uint64   `json:"receivetxs" bson:"receivetxs"`
	LastScanTxID     string              `json:"lastscantxid" bson:"lastscantxid"`
}

type KeyInfoDataV2 KeyInfoData

func NewKeyInfoData(Pubkey, OTAKey string, coinIdx map[string]CoinInfo) *KeyInfoData {
	return &KeyInfoData{
		Pubkey: Pubkey, OTAKey: OTAKey, CoinIndex: coinIdx,
	}
}

func (model *KeyInfoData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *KeyInfoData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type CoinPendingData struct {
	mgm.DefaultModel `bson:",inline"`
	Keyimages        []string `json:"keyimage" bson:"keyimage"`
	ShardID          int      `json:"shardid" bson:"shardid"`
	Locktime         int64    `json:"locktime" bson:"locktime"`
	TxData           string   `json:"txdata" bson:"txdata"`
	TxHash           string   `json:"txhash" bson:"txhash"`
}

func NewCoinPendingData(keyimages []string, shardID int, txHash, txdata string, locktime int64) *CoinPendingData {
	return &CoinPendingData{
		Keyimages: keyimages, ShardID: shardID, TxHash: txHash, TxData: txdata, Locktime: locktime,
	}
}

func (model *CoinPendingData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *CoinPendingData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TokenInfoData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Name             string `json:"name" bson:"name"`
	Symbol           string `json:"symbol" bson:"symbol"`
	Image            string `json:"image" bson:"image"`
	Amount           string `json:"amount" bson:"amount"`
	IsPrivacy        bool   `json:"isprivacy" bson:"isprivacy"`
	IsBridge         bool   `json:"isbridge" bson:"isbridge"`
	IsNFT            bool   `json:"isnft" bson:"isnft"`
	ExternalID       string `json:"externalid" bson:"externalid"`
}

func NewTokenInfoData(tokenID, name, symbol, image string, isprivacy, isbridge bool, amount uint64, isNFT bool, externalid string) *TokenInfoData {
	return &TokenInfoData{
		TokenID: tokenID, Name: name, Symbol: symbol, Image: image, IsPrivacy: isprivacy, IsBridge: isbridge, Amount: strconv.FormatUint(amount, 10), IsNFT: isNFT, ExternalID: externalid,
	}
}

func (model *TokenInfoData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TokenInfoData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type SubmittedOTAKeyData struct {
	mgm.DefaultModel `bson:",inline"`
	OTAKey           string `json:"otakey" bson:"otakey"`
	Pubkey           string `json:"pubkey" bson:"pubkey"`
	Fullkey          string `json:"fullkey" bson:"fullkey"`
	IndexerID        int    `json:"indexerid" bson:"indexerid"`
}

func NewSubmittedOTAKeyData(OTAkey, pubkey, fullkey string, indexerID int) *SubmittedOTAKeyData {
	return &SubmittedOTAKeyData{
		OTAKey: OTAkey, Pubkey: pubkey, Fullkey: fullkey, IndexerID: indexerID,
	}
}

func (model *SubmittedOTAKeyData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *SubmittedOTAKeyData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TxData struct {
	mgm.DefaultModel `bson:",inline"`
	KeyImages        []string `json:"keyimages" bson:"keyimages"`
	PubKeyReceivers  []string `json:"pubkeyreceivers" bson:"pubkeyreceivers"`
	TxHash           string   `json:"txhash" bson:"txhash"`
	TxVersion        int      `json:"txversion" bson:"txversion"`
	TxType           string   `json:"txtype" bson:"txtype"`
	TxDetail         string   `json:"txdetail" bson:"txdetail"`
	TokenID          string   `json:"tokenid" bson:"tokenid"`
	RealTokenID      string   `json:"realtokenid" bson:"realtokenid"`
	BlockHash        string   `json:"blockhash" bson:"blockhash"`
	BlockHeight      uint64   `json:"blockheight" bson:"blockheight"`
	ShardID          int      `json:"shardid" bson:"shardid"`
	Locktime         int64    `json:"locktime" bson:"locktime"`
	Metatype         string   `json:"metatype" bson:"metatype"`
	Metadata         string   `json:"metadata" bson:"metadata"`
	IsNFT            bool     `json:"isnft" bson:"isnft"`
}

func NewTxData(locktime int64, shardID, txVersion int, blockHeight uint64, blockhash, tokenID, txHash, txType, txDetail, metatype, metadata string, keyimages, pubKeyReceivers []string, isNFT bool) *TxData {
	return &TxData{
		TxVersion:       txVersion,
		KeyImages:       keyimages,
		PubKeyReceivers: pubKeyReceivers,
		TxHash:          txHash,
		TxType:          txType,
		TxDetail:        txDetail,
		TokenID:         tokenID,
		ShardID:         shardID,
		BlockHash:       blockhash,
		BlockHeight:     blockHeight,
		Locktime:        locktime,
		Metatype:        metatype,
		Metadata:        metadata,
		IsNFT:           isNFT,
	}
}

func (model *TxData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TxData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PDEStateData struct {
	mgm.DefaultModel `bson:",inline"`
	Version          int    `json:"version" bson:"version"`
	State            string `json:"state" bson:"state"`
	Height           uint64 `json:"height" bson:"height"`
}

func (model *PDEStateData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PDEStateData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

func NewPDEStateData(state string, version int) *PDEStateData {
	return &PDEStateData{
		Version: version,
		State:   state,
	}
}

type ShieldData struct {
	mgm.DefaultModel `bson:",inline"`
	Bridge           string `json:"bridge" bson:"bridge"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Amount           uint64 `json:"amount" bson:"amount"`
	RespondTx        string `json:"respondtx" bson:"respondtx"`
	RequestTx        string `json:"requesttx" bson:"requesttx"`
	IsDecentralized  bool   `json:"isdecentralized" bson:"isdecentralized"`
	Pubkey           string `json:"pubkey" bson:"pubkey"`
	BeaconHeight     uint64 `json:"height" bson:"height"`
	RequestTime      int64  `json:"requesttime" bson:"requesttime"`
}

func NewShieldData(requestTx, respondTx, tokenID, bridge, pubkey string, isDecentralized bool, amount, height uint64, requestTime int64) *ShieldData {
	return &ShieldData{
		RespondTx:       respondTx,
		RequestTx:       requestTx,
		Bridge:          bridge,
		TokenID:         tokenID,
		Amount:          amount,
		IsDecentralized: isDecentralized,
		Pubkey:          pubkey,
		BeaconHeight:    height,
		RequestTime:     requestTime,
	}
}

func (model *ShieldData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}
	return nil
}
func (model *ShieldData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}
	return nil
}

type ContributionData struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTxs       []string `json:"requesttxs" bson:"requesttxs"`
	RespondTxs       []string `json:"respondtxs" bson:"respondtxs"`
	PairID           string   `json:"pairid" bson:"pairid"`
	PoolID           string   `json:"poolid" bson:"poolid"`
	PairHash         string   `json:"pairhash" bson:"pairhash"`
	ContributeTokens []string `json:"contributetokens" bson:"contributetokens"`
	ContributeAmount []uint64 `json:"contributeamount" bson:"contributeamount"`
	ReturnTokens     []string `json:"returntokens" bson:"returntokens"`
	ReturnAmount     []uint64 `json:"returnamount" bson:"returnamount"`
	Contributor      string   `json:"contributor" bson:"contributor"`
	NFTID            string   `json:"nftid" bson:"nftid"`
	RequestTime      int64    `json:"requesttime" bson:"requesttime"`
	Status           string   `json:"status" bson:"status"`
}

// func NewContributionData(requestTx, respondTx, pairID, contributorAddressStr string, respondBlock uint64) *ContributionData {
// 	return &ContributionData{
// 		RequestTx: requestTx, RespondTx: respondTx, PairID: pairID, ContributorAddressStr: contributorAddressStr, Respondblock: respondBlock,
// 	}
// }

func (model *ContributionData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *ContributionData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type WithdrawContributionData struct {
	mgm.DefaultModel      `bson:",inline"`
	Status                int      `json:"status" bson:"status"`
	RequestTx             string   `json:"requesttx" bson:"requesttx"`
	RespondTxs            []string `json:"respondtxs" bson:"respondtxs"`
	WithdrawTokens        []string `json:"withdrawtokens" bson:"withdrawtokens"`
	WithdrawAmount        []uint64 `json:"withdrawamount" bson:"withdrawamount"`
	ShareAmount           uint64   `json:"shareamount" bson:"shareamount"`
	ContributorAddressStr string   `json:"contributor" bson:"contributor"`
	RequestTime           int64    `json:"requesttime" bson:"requesttime"`
	NFTID                 string   `json:"nftid" bson:"nftid"`
	PoolID                string   `json:"poolid" bson:"poolid"`
}

func (model *WithdrawContributionData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *WithdrawContributionData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type WithdrawContributionFeeData struct {
	mgm.DefaultModel      `bson:",inline"`
	RequestTx             string   `json:"requesttx" bson:"requesttx"`
	RespondTxs            []string `json:"respondtxs" bson:"respondtxs"`
	Status                int      `json:"status" bson:"status"`
	PoodID                string   `json:"poolid" bson:"poolid"`
	WithdrawTokens        []string `json:"withdrawtokens" bson:"withdrawtokens"`
	WithdrawAmount        []uint64 `json:"withdrawamount" bson:"withdrawamount"`
	ContributorAddressStr string   `json:"contributor" bson:"contributor"`
	RequestTime           int64    `json:"requesttime" bson:"requesttime"`
	NFTID                 string   `json:"nftid" bson:"nftid"`
}

func (model *WithdrawContributionFeeData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *WithdrawContributionFeeData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TradeOrderWithdrawInfo struct {
	TokenIDs      []string `json:"tokenids" bson:"tokenids"`
	Amount        uint64   `json:"amount" bson:"amount"`
	Status        []int    `json:"status" bson:"status"`
	IsRejected    bool     `json:"isrejected" bson:"isrejected"`
	Responds      []string `json:"responds" bson:"responds"`
	RespondTokens []string `json:"respondtokens" bson:"respondtokens"`
	RespondAmount []uint64 `json:"respondamount" bson:"respondamount"`
}
type TradeOrderData struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTx        string                            `json:"requesttx" bson:"requesttx"`
	WithdrawTxs      []string                          `json:"withdrawtxs" bson:"withdrawtxs"`
	WithdrawInfos    map[string]TradeOrderWithdrawInfo `json:"withdrawinfos" bson:"withdrawinfos"`
	RespondTxs       []string                          `json:"respondtxs" bson:"respondtxs"`
	RespondTokens    []string                          `json:"respondtokens" bson:"respondtokens"`
	RespondAmount    []uint64                          `json:"respondamount" bson:"respondamount"`
	Status           int                               `json:"status" bson:"status"`
	SellTokenID      string                            `json:"selltokenid" bson:"selltokenid"`
	BuyTokenID       string                            `json:"buytokenid" bson:"buytokenid"`
	PairID           string                            `json:"pairid" bson:"pairid"`
	PoolID           string                            `json:"poolid" bson:"poolid"`
	MinAccept        uint64                            `json:"minaccept" bson:"minaccept"`
	Amount           uint64                            `json:"amount" bson:"amount"`
	Requesttime      int64                             `json:"requesttime" bson:"requesttime"`
	NFTID            string                            `json:"nftid" bson:"nftid"`
	Receiver         string                            `json:"receiver" bson:"receiver"`
	ShardID          int                               `json:"shardid" bson:"shardid"`
	BlockHeight      uint64                            `json:"blockheight" bson:"blockheight"`
	Fee              uint64                            `json:"fee" bson:"fee"`
	FeeToken         string                            `json:"feetoken" bson:"feetoken"`
	IsSwap           bool                              `json:"isswap" bson:"isswap"`
	Version          int                               `json:"version" bson:"version"`
	TradingPath      []string                          `json:"tradingpath" bson:"tradingpath"`
}

func NewTradeOrderData(requestTx, selltoken, buytoken, poolid, pairid, nftid string, status int, minAccept, amount uint64, requestTime int64, shardID int, blockHeight uint64) *TradeOrderData {
	return &TradeOrderData{
		NFTID:         nftid,
		RequestTx:     requestTx,
		SellTokenID:   selltoken,
		BuyTokenID:    buytoken,
		Status:        status,
		PoolID:        poolid,
		PairID:        pairid,
		MinAccept:     minAccept,
		Amount:        amount,
		Requesttime:   requestTime,
		ShardID:       shardID,
		BlockHeight:   blockHeight,
		WithdrawInfos: map[string]TradeOrderWithdrawInfo{},
		WithdrawTxs:   []string{},
		RespondTxs:    []string{},
		RespondTokens: []string{},
		RespondAmount: []uint64{},
	}
}

func (model *TradeOrderData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TradeOrderData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PairData struct {
	mgm.DefaultModel `bson:",inline"`
	PairID           string `json:"pairid" bson:"pairid"`
	TokenID1         string `json:"tokenid1" bson:"tokenid1"`
	TokenID2         string `json:"tokenid2" bson:"tokenid2"`
	Token1Amount     uint64 `json:"token1amount" bson:"token1amount"`
	Token2Amount     uint64 `json:"token2amount" bson:"token2amount"`
	PoolCount        int    `json:"poolcount" bson:"poolcount"`
}

func NewPairData(pairid, token1, token2 string, poolcount int, token1value, token2value uint64) *PairData {
	return &PairData{
		PairID:       pairid,
		TokenID1:     token1,
		TokenID2:     token2,
		Token1Amount: token1value,
		Token2Amount: token2value,
		PoolCount:    poolcount,
	}
}

func (model *PairData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PairData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolPairData struct {
	mgm.DefaultModel `bson:",inline"`
	PoolID           string `json:"poolid" bson:"poolid"`
	PairID           string `json:"pairid" bson:"pairid"`
	TokenID1         string `json:"tokenid1" bson:"tokenid1"`
	TokenID2         string `json:"tokenid2" bson:"tokenid2"`
	AMP              uint   `json:"amp" bson:"amp"`
	Token1Amount     uint64 `json:"token1amount" bson:"token1amount"`
	Token2Amount     uint64 `json:"token2amount" bson:"token2amount"`
	Virtual1Amount   uint64 `json:"virtual1amount" bson:"virtual1amount"`
	Virtual2Amount   uint64 `json:"virtual2amount" bson:"virtual2amount"`
	TotalShare       uint64 `json:"totalshare" bson:"totalshare"`
	Version          int    `json:"version" bson:"version"`
}

func NewPoolPairData(poolid, pairid, token1, token2 string, amp uint, token1amount, token2amount uint64) *PoolPairData {
	return &PoolPairData{
		PoolID:       poolid,
		PairID:       pairid,
		TokenID1:     token1,
		TokenID2:     token2,
		AMP:          amp,
		Token1Amount: token1amount,
		Token2Amount: token2amount,
	}
}

func (model *PoolPairData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolPairData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolShareData struct {
	mgm.DefaultModel `bson:",inline"`
	PoolID           string            `json:"poolid" bson:"poolid"`
	NFTID            string            `json:"nftid" bson:"nftid"`
	Amount           uint64            `json:"amount" bson:"amount"`
	TradingFee       map[string]uint64 `json:"tradingfee" bson:"tradingfee"`
	Version          int               `json:"version" bson:"version"`
}

func (model *PoolShareData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolShareData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolStakeHistoryData struct {
	mgm.DefaultModel `bson:",inline"`
	IsStaking        bool   `json:"isstaking" bson:"isstaking"`
	RequestTx        string `json:"requesttx" bson:"requesttx"`
	RespondTx        string `json:"respondtx" bson:"respondtx"`
	Status           int    `json:"status" bson:"status"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	NFTID            string `json:"nftid" bson:"nftid"`
	Amount           uint64 `json:"amount" bson:"amount"`
	Requesttime      int64  `json:"requesttime" bson:"requesttime"`
}

func (model *PoolStakeHistoryData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolStakeHistoryData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolStakeRewardHistoryData struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTx        string   `json:"requesttx" bson:"requesttx"`
	RespondTxs       []string `json:"respondtxs" bson:"respondtxs"`
	RewardTokens     []string `json:"rewardtokens" bson:"rewardtokens"`
	Status           int      `json:"status" bson:"status"`
	TokenID          string   `json:"tokenid" bson:"tokenid"`
	NFTID            string   `json:"nftid" bson:"nftid"`
	Amount           []uint64 `json:"amount" bson:"amount"`
	Requesttime      int64    `json:"requesttime" bson:"requesttime"`
}

func (model *PoolStakeRewardHistoryData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolStakeRewardHistoryData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolStakeData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Amount           uint64 `json:"amount" bson:"amount"`
}

func (model *PoolStakeData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolStakeData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PoolStakerData struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string            `json:"tokenid" bson:"tokenid"`
	NFTID            string            `json:"nftid" bson:"nftid"`
	Amount           uint64            `json:"amount" bson:"amount"`
	Reward           map[string]uint64 `json:"reward" bson:"reward"`
}

func (model *PoolStakerData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PoolStakerData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type ProcessorState struct {
	mgm.DefaultModel `bson:",inline"`
	Processor        string `json:"processor" bson:"processor"`
	State            string `json:"state" bson:"state"`
}

func (model *ProcessorState) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *ProcessorState) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type LimitOrderStatus struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTx        string `json:"requesttx" bson:"requesttx"`
	Token1Balance    uint64 `json:"token1balance" bson:"token1balance"`
	Token2Balance    uint64 `json:"token2balance" bson:"token2balance"`
	Direction        byte   `json:"direction" bson:"direction"`
}

type InstructionBeaconData struct {
	mgm.DefaultModel `bson:",inline"`
	Metatype         string `json:"metatype" bson:"metatype"`
	TxRequest        string `json:"txrequest" bson:"txrequest"`
	Content          string `json:"content" bson:"content"`
	Status           string `json:"status" bson:"status"`
}

func (model *InstructionBeaconData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *InstructionBeaconData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type ClientAssistantData struct {
	mgm.DefaultModel `bson:",inline"`
	DataName         string `json:"dataname" bson:"dataname"`
	Data             string `json:"data" bson:"data"`
}

func (model *ClientAssistantData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *ClientAssistantData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TokenPrice struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	TokenName        string `json:"name" bson:"name"`
	TokenSymbol      string `json:"symbol" bson:"symbol"`
	Price            uint64 `json:"price" bson:"price"`
	Time             int64  `json:"time" bson:"time"`
}

func (model *TokenPrice) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TokenPrice) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PairRanking struct {
	mgm.DefaultModel `bson:",inline"`
	PairID           string `json:"pairid" bson:"pairid"`
	Value            uint64 `json:"value" bson:"value"`
	LeadPool         string `json:"leadpool" bson:"leadpool"`
}

func (model *PairRanking) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PairRanking) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TokenMarketCap struct {
	mgm.DefaultModel `bson:",inline"`
	TokenSymbol      string `json:"symbol" bson:"symbol"`
	Value            uint64 `json:"value" bson:"value"`
	Rank             int    `json:"rank" bson:"rank"`
	PriceChange      string `json:"pricechange" bson:"pricechange"`
}

func (model *TokenMarketCap) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TokenMarketCap) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type RewardRecord struct {
	mgm.DefaultModel `bson:",inline"`
	DataID           string `json:"dataid" bson:"dataid"`
	Data             string `json:"data" bson:"data"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
}

func (model *RewardRecord) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *RewardRecord) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type RewardAPYTracking struct {
	mgm.DefaultModel `bson:",inline"`
	DataID           string `json:"dataid" bson:"dataid"`
	APY              uint64 `json:"apy" bson:"apy"`
	BeaconHeight     uint64 `json:"beaconheight" bson:"beaconheight"`
}

func (model *RewardAPYTracking) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *RewardAPYTracking) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type TokenPdecimal struct {
	mgm.DefaultModel `bson:",inline"`
	TokenID          string `json:"TokenID" bson:"tokenid"`
	Symbol           string `json:"Symbol" bson:"symbol"`
	PSymbol          string `json:"PSymbol" bson:"psymbol"`
	PDecimals        uint64 `json:"PDecimals" bson:"pdecimals"`
}

func (model *TokenPdecimal) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TokenPdecimal) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}
