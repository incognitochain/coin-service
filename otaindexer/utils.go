package otaindexer

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"

	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
)

func doesCoinBelongToKeySet(c *coin.CoinV2, keySet *incognitokey.KeySet, tokenIDs map[string]string, nftIDs map[string]string, willCheckToken bool) (bool, string, *operation.Point, bool) {
	_, txOTARandomPoint, index, err1 := c.GetTxRandomDetail()
	if err1 != nil {
		log.Println(err1)
		return false, "", nil, false
	}
	assetTag := c.GetAssetTag()
	tokenID := ""
	pass := false
	isNFT := false
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
		return pass, "", nil, false
	}
	if assetTag == nil {
		tokenID = common.PRVCoinID.String()
	}
	tokenCount := lastTokenListCount
retryCheckTokenID:
	if assetTag != nil && tokenID == "" {
	retryGetToken:
		if tokenCount == 0 {
			err := retrieveTokenIDList()
			if err != nil {
				log.Println("retrieveTokenIDList", err)
				time.Sleep(1 * time.Second)
				goto retryGetToken
			}
		}
		tokenListLock.RLock()
		tokenIDs = make(map[string]string)
		for k, v := range lastTokenIDMap {
			tokenIDs[k] = v
		}
		nftIDs = make(map[string]string)
		for k, v := range lastNFTIDMap {
			nftIDs[k] = v
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
				return false, "", nil, false
			}
			for tkStr, tkID := range tokenIDs {
				recomputedAssetTag, err := new(operation.Point).UnmarshalText([]byte(tkStr))
				if err != nil {
					log.Println("UnmarshalText tkStr", err)
					return false, "", nil, false
				}
				recomputedAssetTag.Add(recomputedAssetTag, new(operation.Point).ScalarMult(operation.PedCom.G[coin.PedersenRandomnessIndex], blinder))
				if operation.IsPointEqual(recomputedAssetTag, assetTag) {
					tokenID = tkID
					break
				}
			}

			if tokenID == "" {
				for tkStr, tkID := range nftIDs {
					recomputedAssetTag, err := new(operation.Point).UnmarshalText([]byte(tkStr))
					if err != nil {
						log.Println("UnmarshalText tkStr", err)
						return false, "", nil, false
					}
					recomputedAssetTag.Add(recomputedAssetTag, new(operation.Point).ScalarMult(operation.PedCom.G[coin.PedersenRandomnessIndex], blinder))
					if operation.IsPointEqual(recomputedAssetTag, assetTag) {
						tokenID = tkID
						isNFT = true
						break
					}
				}
			}
		}
		if tokenID == "" {
			log.Println("retryCheckTokenID")
			tokenCount = 0
			time.Sleep(1 * time.Second)
			goto retryCheckTokenID
		}
	}

	return pass, tokenID, rK, isNFT
}

var tokenListLock sync.RWMutex
var lastTokenListCount int64
var lastTokenIDMap map[string]string
var lastNFTListCount int64
var lastNFTIDMap map[string]string

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

	nftCount, err := database.DBGetNFTCount()
	if err != nil {
		return err
	}
	if nftCount != lastNFTListCount {
		tokenInfos, err := database.DBGetNFTInfo()
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
			lastNFTIDMap[recomputedAssetTag.String()] = tokenInfo.TokenID
		}
		lastNFTListCount = nftCount
		tokenListLock.Unlock()
	}
	return nil
}

func checkPubkeyAndTxRandom(txRandom coin.TxRandom, pubkey operation.Point, keySet *incognitokey.KeySet) bool {
	txRandomOTAPoint, err1 := txRandom.GetTxOTARandomPoint()
	_, err2 := txRandom.GetTxConcealRandomPoint()
	index, err3 := txRandom.GetIndex()
	if err1 != nil || err2 != nil || err3 != nil {
		log.Println("checkPubkeyAndTxRandom", err1, err2, err3)
		return false
	}
	rK := new(operation.Point).ScalarMult(txRandomOTAPoint, keySet.OTAKey.GetOTASecretKey())

	hashed := operation.HashToScalar(
		append(rK.ToBytesS(), common.Uint32ToBytes(index)...),
	)

	HnG := new(operation.Point).ScalarMultBase(hashed)
	KCheck := new(operation.Point).Sub(&pubkey, HnG)
	pass := operation.IsPointEqual(KCheck, keySet.OTAKey.GetPublicSpend())
	return pass
}

func extractPubkeyAndTxRandom(tx shared.TxData) (*coin.TxRandom, *operation.Point, error) {
	metaRaw := tx.Metadata
	var meta metadataPdexv3.TradeRequest
	err := json.UnmarshalFromString(metaRaw, &meta)
	if err != nil {
		return nil, nil, err
	}
	for _, v := range meta.Receiver {
		return &v.TxRandom, &v.PublicKey, nil
	}
	return nil, nil, errors.New("len(meta.Receiver) == 0")
}
