package shared

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/incognitochain/incognito-chain/common"
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
