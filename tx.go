package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

func getTxByHash(fullnode, txHash string) (*TxDetail, error) {
	query := fmt.Sprintf(`{
		"jsonrpc":"1.0",
		"method":"gettransactionbyhash",
		"params":["%s"],
		"id":1
	}`, txHash)
	var jsonStr = []byte(query)
	req, _ := http.NewRequest("POST", fullnode, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	autoTx, txError := ParseAutoTxHashFromBytes(body)
	if txError != nil {
		return nil, err
	}
	return autoTx, nil
}

func ParseAutoTxHashFromBytes(b []byte) (*TxDetail, error) {
	data := new(TxDetail)
	err := json.Unmarshal(b, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type TxDetail struct {
	ID     int `json:"Id"`
	Result struct {
		// BlockHash   string `json:"BlockHash"`
		// BlockHeight int    `json:"BlockHeight"`
		// TxSize      int    `json:"TxSize"`
		// Index       int    `json:"Index"`
		// ShardID     int    `json:"ShardID"`
		Hash string `json:"Hash"`
		// Version     int    `json:"Version"`
		// Type        string `json:"Type"`
		// LockTime    string `json:"LockTime"`
		// Fee         int    `json:"Fee"`
		// Image       string `json:"Image"`
		// IsPrivacy   bool   `json:"IsPrivacy"`
		// Proof       string `json:"Proof"`
		ProofDetail struct {
			InputCoins []struct {
				CoinDetails struct {
					PublicKey      string `json:"PublicKey"`
					CoinCommitment string `json:"CoinCommitment"`
					SNDerivator    struct {
					} `json:"SNDerivator"`
					SerialNumber string `json:"SerialNumber"`
					Randomness   struct {
					} `json:"Randomness"`
					Value int    `json:"Value"`
					Info  string `json:"Info"`
				} `json:"CoinDetails"`
				CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
			} `json:"InputCoins"`
			OutputCoins []struct {
				CoinDetails struct {
					PublicKey      string `json:"PublicKey"`
					CoinCommitment string `json:"CoinCommitment"`
					SNDerivator    struct {
					} `json:"SNDerivator"`
					SerialNumber string `json:"SerialNumber"`
					Randomness   struct {
					} `json:"Randomness"`
					Value int    `json:"Value"`
					Info  string `json:"Info"`
				} `json:"CoinDetails"`
				CoinDetailsEncrypted string `json:"CoinDetailsEncrypted"`
			} `json:"OutputCoins"`
		} `json:"ProofDetail"`
		InputCoinPubKey string `json:"InputCoinPubKey"`
		// SigPubKey                     string `json:"SigPubKey"`
		// Sig                           string `json:"Sig"`
		// Metadata                      string `json:"Metadata"`
		// CustomTokenData               string `json:"CustomTokenData"`
		// PrivacyCustomTokenID          string `json:"PrivacyCustomTokenID"`
		// PrivacyCustomTokenName        string `json:"PrivacyCustomTokenName"`
		// PrivacyCustomTokenSymbol      string `json:"PrivacyCustomTokenSymbol"`
		// PrivacyCustomTokenData        string `json:"PrivacyCustomTokenData"`
		// PrivacyCustomTokenProofDetail struct {
		// 	InputCoins  interface{} `json:"InputCoins"`
		// 	OutputCoins interface{} `json:"OutputCoins"`
		// } `json:"PrivacyCustomTokenProofDetail"`
		// PrivacyCustomTokenIsPrivacy bool   `json:"PrivacyCustomTokenIsPrivacy"`
		// PrivacyCustomTokenFee       int    `json:"PrivacyCustomTokenFee"`
		// IsInMempool                 bool   `json:"IsInMempool"`
		// IsInBlock                   bool   `json:"IsInBlock"`
		// Info                        string `json:"Info"`
	} `json:"Result"`
	Error   interface{} `json:"Error"`
	Params  []string    `json:"Params"`
	Method  string      `json:"Method"`
	Jsonrpc string      `json:"Jsonrpc"`
}
