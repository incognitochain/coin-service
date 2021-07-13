package shared

import (
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

type TransactionDetail struct {
	BlockHash   string `json:"BlockHash"`
	BlockHeight uint64 `json:"BlockHeight"`
	TxSize      uint64 `json:"TxSize"`
	Index       uint64 `json:"Index"`
	ShardID     byte   `json:"ShardID"`
	Hash        string `json:"Hash"`
	Version     int8   `json:"Version"`
	Type        string `json:"Type"` // Transaction type
	LockTime    string `json:"LockTime"`
	Fee         uint64 `json:"Fee"` // Fee applies: always consant
	Image       string `json:"Image"`

	IsPrivacy        bool                   `json:"IsPrivacy"`
	Proof            privacy.Proof          `json:"Proof"`
	ProofDetail      jsonresult.ProofDetail `json:"ProofDetail"`
	InputCoinPubKey  string                 `json:"InputCoinKeyImage"`
	OutputCoinPubKey []string               `json:"OutputCoinPubKey"`
	OutputCoinSND    []string               `json:"OutputCoinSND"`

	TokenInputCoinPubKey  string   `json:"TokenInputCoinKeyImage"`
	TokenOutputCoinPubKey []string `json:"TokenOutputCoinPubKey"`
	TokenOutputCoinSND    []string `json:"TokenOutputCoinSND"`

	SigPubKey string `json:"SigPubKey,omitempty"` // 64 bytes
	Sig       string `json:"Sig,omitempty"`       // 64 bytes

	Metadata                      string                 `json:"Metadata"`
	CustomTokenData               string                 `json:"CustomTokenData"`
	PrivacyCustomTokenID          string                 `json:"PrivacyCustomTokenID"`
	PrivacyCustomTokenName        string                 `json:"PrivacyCustomTokenName"`
	PrivacyCustomTokenSymbol      string                 `json:"PrivacyCustomTokenSymbol"`
	PrivacyCustomTokenData        string                 `json:"PrivacyCustomTokenData"`
	PrivacyCustomTokenProofDetail jsonresult.ProofDetail `json:"PrivacyCustomTokenProofDetail"`
	PrivacyCustomTokenIsPrivacy   bool                   `json:"PrivacyCustomTokenIsPrivacy"`
	PrivacyCustomTokenFee         uint64                 `json:"PrivacyCustomTokenFee"`

	IsInMempool bool `json:"IsInMempool"`
	IsInBlock   bool `json:"IsInBlock"`

	Info string `json:"Info"`
}
