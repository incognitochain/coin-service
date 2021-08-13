package otaindexer

import (
	"log"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
)

func doesCoinBelongToKeySet(c *coin.CoinV2, keySet *incognitokey.KeySet, tokenIDs map[string]string, willCheckToken bool) (bool, string, *operation.Point) {
	_, txOTARandomPoint, index, err1 := c.GetTxRandomDetail()
	if err1 != nil {
		log.Println(err1)
		return false, "", nil
	}
	assetTag := c.GetAssetTag()
	tokenID := ""
	pass := false

	otasecret := keySet.OTAKey.GetOTASecretKey()
	pubkey := c.GetPublicKey()
	otapub := keySet.OTAKey.GetPublicSpend()

	rK := new(operation.Point).ScalarMult(txOTARandomPoint, otasecret)

	hashed := operation.HashToScalar(
		append(rK.ToBytesS(), common.Uint32ToBytes(index)...),
	)

	HnG := new(operation.Point).ScalarMultBase(hashed)
	KCheck := new(operation.Point).Sub(pubkey, HnG)
	pass = operation.IsPointEqual(KCheck, otapub)
	if !willCheckToken {
		return pass, "", nil
	}
	if assetTag == nil {
		tokenID = common.PRVCoinID.String()
	}

	tokenMapLen := len(lastTokenIDMap)
retryCheckTokenID:
	if assetTag != nil && tokenID == "" {
	retryGetToken:
		if tokenMapLen == 0 {
			err := retrieveTokenIDList()
			if err != nil {
				log.Println("retrieveTokenIDList", err)
				time.Sleep(1 * time.Second)
				goto retryGetToken
			}
			tokenMapLen = len(lastTokenIDMap)
		}
		tokenListLock.RLock()
		tokenIDs = make(map[string]string)
		for k, v := range lastTokenIDMap {
			tokenIDs[k] = v
		}
		tokenListLock.RUnlock()
	}
	if pass && assetTag != nil && len(tokenIDs) != 0 {
		if tk, ok := tokenIDs[assetTag.String()]; ok {
			tokenID = tk
		} else {
			blinder, err := coin.ComputeAssetTagBlinder(rK)
			if err != nil {
				log.Println(err)
				return false, "", nil
			}
			for tkStr, tkID := range tokenIDs {
				recomputedAssetTag, err := new(operation.Point).UnmarshalText([]byte(tkStr))
				if err != nil {
					log.Println("UnmarshalText tkStr", err)
					return false, "", nil
				}
				recomputedAssetTag.Add(recomputedAssetTag, new(operation.Point).ScalarMult(operation.PedCom.G[coin.PedersenRandomnessIndex], blinder))
				if operation.IsPointEqual(recomputedAssetTag, assetTag) {
					tokenID = tkID
					break
				}
			}
		}
		if tokenID == "" {
			log.Println("retryCheckTokenID")
			tokenMapLen = 0
			time.Sleep(1 * time.Second)
			goto retryCheckTokenID
		}
	}

	return pass, tokenID, rK
}

var lastTokenListCount int64
var tokenListLock sync.RWMutex
var lastTokenIDMap map[string]string

func retrieveTokenIDList() error {
	if len(lastTokenIDMap) == 0 {
		lastTokenIDMap = make(map[string]string)
	}
	tokenCount, err := database.DBGetTokenCount()
	if err != nil {
		return err
	}
	if tokenCount != lastTokenListCount {
		tokenInfos, err := database.DBGetTokenInfo()
		if err != nil {
			return err
		}
		tokenListLock.Lock()
		for _, tokenInfo := range tokenInfos {
			tokenID, err := new(common.Hash).NewHashFromStr(tokenInfo.TokenID)
			if err != nil {
				return err
			}
			recomputedAssetTag := operation.HashToPoint(tokenID[:])
			lastTokenIDMap[recomputedAssetTag.String()] = tokenInfo.TokenID
		}
		lastTokenListCount = tokenCount
		tokenListLock.Unlock()
	}
	return nil
}
