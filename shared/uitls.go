package shared

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/transaction/utils"
	"github.com/incognitochain/incognito-chain/wallet"
)

func OTAKeyFromRaw(b []byte) privacy.OTAKey {
	result := &privacy.OTAKey{}
	result.SetOTASecretKey(b[0:32])
	result.SetPublicSpend(b[32:64])
	return *result
}

func TokenIDStringToHash(tokenID []string) ([]*common.Hash, error) {
	var result []*common.Hash
	return result, nil
}

func AssetTagStringToPoint(assetTags []string) ([]*operation.Point, error) {
	var result []*operation.Point
	for _, assetTag := range assetTags {
		assetTagBytes, err := hex.DecodeString(assetTag)
		if err != nil {
			return nil, err
		}
		assetTagPoint, err := new(operation.Point).FromBytesS(assetTagBytes)
		if err != nil {
			return nil, err
		}
		result = append(result, assetTagPoint)
	}
	return result, nil
}

func CalculateSharedSecret(txOTARandomPointList []string, otakey string) ([]*operation.Point, error) {
	var result []*operation.Point
	wl, err := wallet.Base58CheckDeserialize(otakey)
	if err != nil {
		return nil, err
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		return nil, errors.New("OTASecretKey is invalid")
	}
	for _, txOTARandomPoint := range txOTARandomPointList {
		randPointBytes, err := hex.DecodeString(txOTARandomPoint)
		if err != nil {
			return nil, err
		}
		randPoint, err := new(operation.Point).FromBytesS(randPointBytes)
		if err != nil {
			return nil, err
		}
		rK := new(operation.Point).ScalarMult(randPoint, wl.KeySet.OTAKey.GetOTASecretKey())
		result = append(result, rK)
	}
	return result, nil
}

func CheckTokenIDWithOTA(sharedSecret, assetTag *operation.Point, tokenID *common.Hash) (bool, error) {
	recomputedAssetTag := operation.HashToPoint(tokenID[:])
	if operation.IsPointEqual(recomputedAssetTag, assetTag) {
		return true, nil
	}

	blinder, err := coin.ComputeAssetTagBlinder(sharedSecret)
	if err != nil {
		return false, err
	}

	recomputedAssetTag.Add(recomputedAssetTag, new(operation.Point).ScalarMult(operation.PedCom.G[coin.PedersenRandomnessIndex], blinder))
	if operation.IsPointEqual(recomputedAssetTag, assetTag) {
		return true, nil
	}
	return false, nil
}

// DeserializeTransactionJSON parses a transaction from raw JSON into a TxChoice object.
// It covers all transaction types.
func DeserializeTransactionJSON(data []byte) (*transaction.TxChoice, error) {
	result := &transaction.TxChoice{}
	holder := make(map[string]interface{})
	err := json.Unmarshal(data, &holder)
	if err != nil {
		return nil, err
	}

	// Used to parse json
	type txJsonDataVersion struct {
		Version int8 `json:"Version"`
		Type    string
	}
	_, isTokenTx := holder["TxTokenPrivacyData"]
	_, hasVersionOutside := holder["Version"]
	var verHolder txJsonDataVersion
	json.Unmarshal(data, &verHolder)
	if hasVersionOutside {
		switch verHolder.Version {
		case utils.TxVersion1Number:
			if isTokenTx {
				// token ver 1
				result.TokenVersion1 = &transaction.TxTokenVersion1{}
				err := json.Unmarshal(data, result.TokenVersion1)
				return result, err
			} else {
				// tx ver 1
				result.Version1 = &transaction.TxVersion1{}
				err := json.Unmarshal(data, result.Version1)
				return result, err
			}
		case utils.TxVersion2Number: // the same as utils.TxConversionVersion12Number
			if isTokenTx {
				// rejected
				return nil, errors.New("Error unmarshalling TX from JSON : misplaced version")
			} else {
				// tx ver 2
				result.Version2 = &transaction.TxVersion2{}
				err := json.Unmarshal(data, result.Version2)
				return result, err
			}
		default:
			return nil, errors.New(fmt.Sprintf("Error unmarshalling TX from JSON : wrong version of %d", verHolder.Version))
		}
	} else {
		if isTokenTx {
			// token ver 2
			result.TokenVersion2 = &transaction.TxTokenVersion2{}
			err := json.Unmarshal(data, result.TokenVersion2)
			return result, err
		} else {
			return nil, errors.New("Error unmarshalling TX from JSON")
		}
	}

}

func NewTransactionDetail(tx metadata.Transaction, blockHash *common.Hash, blockHeight uint64, index int, shardID byte, base58Fmt bool) (*TransactionDetail, error) {
	var result *TransactionDetail
	blockHashStr := ""
	if blockHash != nil {
		blockHashStr = blockHash.String()
	}
	switch tx.GetType() {
	case common.TxNormalType, common.TxRewardType, common.TxReturnStakingType, common.TxConversionType:
		{
			var sigPubKeyStr string
			txVersion := tx.GetVersion()
			if txVersion == 1 {
				sigPubKeyStr = base58.Base58Check{}.Encode(tx.GetSigPubKey(), 0x0)
			} else {
				sigPubKey := new(transaction.TxSigPubKeyVer2)
				if err := sigPubKey.SetBytes(tx.GetSigPubKey()); err != nil {
					sigPubKeyStr = "[]"
				} else {
					if temp, err := json.Marshal(sigPubKey); err != nil {
						sigPubKeyStr = "[]"
					} else {
						sigPubKeyStr = string(temp)
					}
				}
			}

			result = &TransactionDetail{
				BlockHash:   blockHashStr,
				BlockHeight: blockHeight,
				Index:       uint64(index),
				TxSize:      tx.GetTxActualSize(),
				ShardID:     shardID,
				Hash:        tx.Hash().String(),
				Version:     tx.GetVersion(),
				Type:        tx.GetType(),
				LockTime:    time.Unix(tx.GetLockTime(), 0).Format(common.DateOutputFormat),
				Fee:         tx.GetTxFee(),
				IsPrivacy:   tx.IsPrivacy(),
				Proof:       tx.GetProof(),
				SigPubKey:   sigPubKeyStr,
				Sig:         base58.Base58Check{}.Encode(tx.GetSig(), 0x0),
				Info:        string(tx.GetInfo()),
			}
			if result.Proof != nil {
				inputCoins := result.Proof.GetInputCoins()
				outputCoins := result.Proof.GetOutputCoins()
				if len(inputCoins) > 0 && inputCoins[0].GetPublicKey() != nil {
					if base58Fmt {
						result.InputCoinPubKey = base58.Base58Check{}.Encode(inputCoins[0].GetPublicKey().ToBytesS(), common.ZeroByte)
					} else {
						result.InputCoinPubKey = base64.StdEncoding.EncodeToString(inputCoins[0].GetPublicKey().ToBytesS())
					}
				}
				if len(outputCoins) > 0 {
					for _, coin := range outputCoins {
						if base58Fmt {
							result.OutputCoinPubKey = append(result.OutputCoinPubKey, base58.Base58Check{}.Encode(coin.GetPublicKey().ToBytesS(), common.ZeroByte))
							if coin.GetVersion() == 1 {
								result.OutputCoinSND = append(result.OutputCoinSND, base58.Base58Check{}.Encode(coin.GetSNDerivator().ToBytesS(), common.ZeroByte))
							}
						} else {
							result.OutputCoinPubKey = append(result.OutputCoinPubKey, base64.StdEncoding.EncodeToString(coin.GetPublicKey().ToBytesS()))
							if coin.GetVersion() == 1 {
								result.OutputCoinSND = append(result.OutputCoinSND, base64.StdEncoding.EncodeToString(coin.GetSNDerivator().ToBytesS()))
							}
						}
					}
				}
			}
			meta := tx.GetMetadata()
			if meta != nil {
				metaData, _ := json.Marshal(meta)
				result.Metadata = string(metaData)
			}
			if result.Proof != nil {
				result.ProofDetail.ConvertFromProof(result.Proof)
			}
		}
	case common.TxCustomTokenPrivacyType, common.TxTokenConversionType:
		{
			txToken, ok := tx.(transaction.TransactionToken)
			if !ok {
				return nil, errors.New("cannot detect transaction type")
			}
			txTokenData := transaction.GetTxTokenDataFromTransaction(tx)
			result = &TransactionDetail{
				BlockHash:                blockHashStr,
				BlockHeight:              blockHeight,
				Index:                    uint64(index),
				TxSize:                   tx.GetTxActualSize(),
				ShardID:                  shardID,
				Hash:                     tx.Hash().String(),
				Version:                  tx.GetVersion(),
				Type:                     tx.GetType(),
				LockTime:                 time.Unix(tx.GetLockTime(), 0).Format(common.DateOutputFormat),
				Fee:                      tx.GetTxFee(),
				Proof:                    txToken.GetTxBase().GetProof(),
				SigPubKey:                base58.Base58Check{}.Encode(tx.GetSigPubKey(), 0x0),
				Sig:                      base58.Base58Check{}.Encode(tx.GetSig(), 0x0),
				Info:                     string(tx.GetInfo()),
				IsPrivacy:                tx.IsPrivacy(),
				PrivacyCustomTokenSymbol: txTokenData.PropertySymbol,
				PrivacyCustomTokenName:   txTokenData.PropertyName,
				PrivacyCustomTokenID:     txTokenData.PropertyID.String(),
				PrivacyCustomTokenFee:    txTokenData.TxNormal.GetTxFee(),
			}

			if result.Proof != nil {
				inputCoins := result.Proof.GetInputCoins()
				outputCoins := result.Proof.GetOutputCoins()
				if len(inputCoins) > 0 && inputCoins[0].GetPublicKey() != nil {
					if base58Fmt {
						result.InputCoinPubKey = base58.Base58Check{}.Encode(inputCoins[0].GetPublicKey().ToBytesS(), common.ZeroByte)
					} else {
						result.InputCoinPubKey = base64.StdEncoding.EncodeToString(inputCoins[0].GetPublicKey().ToBytesS())
					}
				}
				if len(outputCoins) > 0 {
					for _, coin := range outputCoins {
						if base58Fmt {
							result.OutputCoinPubKey = append(result.OutputCoinPubKey, base58.Base58Check{}.Encode(coin.GetPublicKey().ToBytesS(), common.ZeroByte))
							if coin.GetVersion() == 1 {
								result.OutputCoinSND = append(result.OutputCoinSND, base58.Base58Check{}.Encode(coin.GetSNDerivator().ToBytesS(), common.ZeroByte))
							}
						} else {
							result.OutputCoinPubKey = append(result.OutputCoinPubKey, base64.StdEncoding.EncodeToString(coin.GetPublicKey().ToBytesS()))
							if coin.GetVersion() == 1 {
								result.OutputCoinSND = append(result.OutputCoinSND, base64.StdEncoding.EncodeToString(coin.GetSNDerivator().ToBytesS()))
							}
						}
					}
				}
			}

			tokenData, _ := json.Marshal(txTokenData)
			result.PrivacyCustomTokenData = string(tokenData)
			if tx.GetMetadata() != nil {
				metaData, _ := json.Marshal(tx.GetMetadata())
				result.Metadata = string(metaData)
			}
			if result.Proof != nil {
				result.ProofDetail.ConvertFromProof(result.Proof)
			}
			result.PrivacyCustomTokenIsPrivacy = txTokenData.TxNormal.IsPrivacy()
			if txTokenData.TxNormal.GetProof() != nil {
				result.PrivacyCustomTokenProofDetail.ConvertFromProof(txTokenData.TxNormal.GetProof())
			}
		}
	default:
		{
			return nil, errors.New("Tx type is invalid")
		}
	}
	return result, nil
}
