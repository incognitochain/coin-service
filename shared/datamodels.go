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
	TxHash           string   `json:"txhash" bson:"txhash"`
}

func NewCoinPendingData(keyimages []string, shardID int, TxHash string) *CoinPendingData {
	return &CoinPendingData{
		Keyimages: keyimages, ShardID: shardID, TxHash: TxHash,
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
}

func NewTokenInfoData(tokenID, name, symbol, image string, isprivacy bool, amount uint64) *TokenInfoData {
	return &TokenInfoData{
		TokenID: tokenID, Name: name, Symbol: symbol, Image: image, IsPrivacy: isprivacy, Amount: strconv.FormatUint(amount, 10),
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
	IndexerID        int    `json:"indexerid" bson:"indexerid"`
}

func NewSubmittedOTAKeyData(OTAkey, pubkey string, indexerID int) *SubmittedOTAKeyData {
	return &SubmittedOTAKeyData{
		OTAKey: OTAkey, Pubkey: pubkey, IndexerID: indexerID,
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
}

func NewTradeData(requestTx, respondTx, status, tokenID string, amount uint64) *TradeData {
	return &TradeData{
		RequestTx: requestTx, RespondTx: respondTx, Status: status, TokenID: tokenID, Amount: amount,
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
}

func NewShieldData(requestTx, respondTx, tokenID, shieldType, bridge string, isDecentralized bool, amount uint64) *ShieldData {
	return &ShieldData{
		RespondTx:       respondTx,
		RequestTx:       requestTx,
		ShieldType:      shieldType,
		Bridge:          bridge,
		TokenID:         tokenID,
		Amount:          amount,
		IsDecentralized: isDecentralized,
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
