package main

import (
	"encoding/hex"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
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
	otakeyBytes, _, err := base58.DecodeCheck(otakey)
	if err != nil {
		return nil, err
	}
	key := &privacy.OTAKey{}
	key.SetOTASecretKey(otakeyBytes[0:32])
	key.SetPublicSpend(otakeyBytes[32:64])
	for _, txOTARandomPoint := range txOTARandomPointList {
		randPointBytes, err := hex.DecodeString(txOTARandomPoint)
		if err != nil {
			return nil, err
		}
		randPoint, err := new(operation.Point).FromBytesS(randPointBytes)
		if err != nil {
			return nil, err
		}
		rK := new(operation.Point).ScalarMult(randPoint, key.GetOTASecretKey())
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
