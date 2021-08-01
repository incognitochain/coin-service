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
	TotalReceiveTxs  map[string]uint64   `json:"receivetxs" bson:"receivetxs"`
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
}

func NewTokenInfoData(tokenID, name, symbol, image string, isprivacy, isbridge bool, amount uint64) *TokenInfoData {
	return &TokenInfoData{
		TokenID: tokenID, Name: name, Symbol: symbol, Image: image, IsPrivacy: isprivacy, IsBridge: isbridge, Amount: strconv.FormatUint(amount, 10),
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
	BlockHeight      string   `json:"blockheight" bson:"blockheight"`
	ShardID          int      `json:"shardid" bson:"shardid"`
	Locktime         int64    `json:"locktime" bson:"locktime"`
	Metatype         string   `json:"metatype" bson:"metatype"`
	Metadata         string   `json:"metadata" bson:"metadata"`
}

func NewTxData(locktime int64, shardID, txVersion int, blockHash, blockHeight, tokenID, txHash, txType, txDetail, metatype, metadata string, keyimages, pubKeyReceivers []string) *TxData {
	return &TxData{
		TxVersion:       txVersion,
		KeyImages:       keyimages,
		PubKeyReceivers: pubKeyReceivers,
		TxHash:          txHash,
		TxType:          txType,
		TxDetail:        txDetail,
		TokenID:         tokenID,
		ShardID:         shardID,
		BlockHash:       blockHash,
		BlockHeight:     blockHeight,
		Locktime:        locktime,
		Metatype:        metatype,
		Metadata:        metadata,
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

type TradeData struct {
	mgm.DefaultModel `bson:",inline"`
	RequestTx        string `json:"requesttx" bson:"requesttx"`
	RespondTx        string `json:"respondtx" bson:"respondtx"`
	Status           string `json:"status" bson:"status"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Amount           uint64 `json:"amount" bson:"amount"`
	PoolID           string `json:"poolid" bson:"poolid"`
}

func NewTradeData(requestTx, respondTx, status, tokenID string, amount uint64, poolID string) *TradeData {
	return &TradeData{
		RequestTx: requestTx, RespondTx: respondTx, Status: status, TokenID: tokenID, Amount: amount, PoolID: poolID,
	}
}

func (model *TradeData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *TradeData) Saving() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Saving(); err != nil {
		return err
	}

	return nil
}

type PDEStateData struct {
	mgm.DefaultModel `bson:",inline"`
	State            string `json:"state" bson:"state"`
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

func NewPDEStateData(state string) *PDEStateData {
	return &PDEStateData{
		State: state,
	}
}

type ShieldData struct {
	mgm.DefaultModel `bson:",inline"`
	Bridge           string `json:"bridge" bson:"bridge"`
	TokenID          string `json:"tokenid" bson:"tokenid"`
	Amount           uint64 `json:"amount" bson:"amount"`
	RespondTx        string `json:"respondtx" bson:"respondtx"`
	RequestTx        string `json:"requesttx" bson:"requesttx"`
	ShieldType       string `json:"shieldtype" bson:"shieldtype"`
	IsDecentralized  bool   `json:"isdecentralized" bson:"isdecentralized"`
	Pubkey           string `json:"pubkey" bson:"pubkey"`
	BeaconHeight     uint64 `json:"height" bson:"height"`
}

func NewShieldData(requestTx, respondTx, tokenID, shieldType, bridge, pubkey string, isDecentralized bool, amount, height uint64) *ShieldData {
	return &ShieldData{
		RespondTx:       respondTx,
		RequestTx:       requestTx,
		ShieldType:      shieldType,
		Bridge:          bridge,
		TokenID:         tokenID,
		Amount:          amount,
		IsDecentralized: isDecentralized,
		Pubkey:          pubkey,
		BeaconHeight:    height,
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

func NewContributionData(requestTx, respondTx, status, pairID, tokenID, contributorAddressStr string, amount, returnamount uint64, respondBlock uint64) *ContributionData {
	return &ContributionData{
		RequestTx: requestTx, RespondTx: respondTx, Status: status, PairID: pairID, TokenID: tokenID, Amount: amount, ReturnAmount: returnamount, ContributorAddressStr: contributorAddressStr, Respondblock: respondBlock,
	}
}

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

func NewWithdrawContributionData(requestTx, status, tokenID1, tokenID2, contributorAddressStr string, respondTx []string, amount1, amount2 uint64, respondTime int64) *WithdrawContributionData {
	return &WithdrawContributionData{
		RequestTx: requestTx, Status: status, RespondTx: respondTx, TokenID1: tokenID1, TokenID2: tokenID2, Amount1: amount1, Amount2: amount2, ContributorAddressStr: contributorAddressStr, RespondTime: respondTime,
	}
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
	RequestTx             string `json:"requesttx" bson:"requesttx"`
	RespondTx             string `json:"respondtx" bson:"respondtx"`
	Status                string `json:"status" bson:"status"`
	TokenID1              string `json:"tokenid1" bson:"tokenid1"`
	TokenID2              string `json:"tokenid2" bson:"tokenid2"`
	Amount                uint64 `json:"amount" bson:"amount"`
	ContributorAddressStr string `json:"contributor" bson:"contributor"`
	RespondTime           int64  `json:"respondtime" bson:"respondtime"`
}

func NewWithdrawContributionFeeData(requestTx, respondTx, status, tokenID1, tokenID2, contributorAddressStr string, amount uint64, respondTime int64) *WithdrawContributionFeeData {
	return &WithdrawContributionFeeData{
		RequestTx: requestTx, RespondTx: respondTx, Status: status, TokenID1: tokenID1, TokenID2: tokenID2, Amount: amount, ContributorAddressStr: contributorAddressStr, RespondTime: respondTime,
	}
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

type PendingTradeOrderData struct {
	mgm.DefaultModel `bson:",inline"`
	Txhash           string `json:"txhash" bson:"txhash"`
	SellTokenID      string `json:"selltokenid" bson:"selltokenid"`
	PoolID           string `json:"poolid" bson:"poolid"`
	PairID           string `json:"pairid" bson:"pairid"`
	Price            uint64 `json:"price" bson:"price"`
	Amount           uint64 `json:"amount" bson:"amount"`
	Remain           uint64 `json:"remain" bson:"remain"`
	Locktime         int64  `json:"locktime" bson:"locktime"`
}

func NewPendingTradeOrderData(txhash, selltoken, poolid, pairid string, price, amount, remain uint64, locktime int64) *PendingTradeOrderData {
	return &PendingTradeOrderData{
		Txhash:      txhash,
		SellTokenID: selltoken,
		PoolID:      poolid,
		PairID:      pairid,
		Price:       price,
		Amount:      amount,
		Remain:      remain,
		Locktime:    locktime,
	}
}

func (model *PendingTradeOrderData) Creating() error {
	// Call the DefaultModel Creating hook
	if err := model.DefaultModel.Creating(); err != nil {
		return err
	}

	return nil
}
func (model *PendingTradeOrderData) Saving() error {
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
	Token1           string `json:"token1" bson:"token1"`
	Token2           string `json:"token2" bson:"token2"`
	AMP              int    `json:"amp" bson:"amp"`
	Token1Amount     uint64 `json:"token1amount" bson:"token1amount"`
	Token2Amount     uint64 `json:"token2amount" bson:"token2amount"`
}

func NewPoolPairData(poolid, pairid, token1, token2 string, amp int, token1amount, token2amount uint64) *PoolPairData {
	return &PoolPairData{
		PoolID:       poolid,
		PairID:       pairid,
		Token1:       token1,
		Token2:       token2,
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
