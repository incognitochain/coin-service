package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/kamva/mgm/v3"
	stats "github.com/semihalev/gin-stats"
)

func startGinService() {
	log.Println("initiating api-service...")

	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(stats.RequestStats())

	r.GET("/stats", func(c *gin.Context) {
		c.JSON(http.StatusOK, stats.Report())
	})
	r.GET("/health", API_HealthCheck)
	// if serviceCfg.Mode == QUERYMODE {
	r.GET("/getcoinspending", API_GetCoinsPending)
	r.GET("/getcoins", API_GetCoins)
	r.GET("/getkeyinfo", API_GetKeyInfo)
	r.POST("/checkkeyimages", API_CheckKeyImages)
	r.POST("/getrandomcommitments", API_GetRandomCommitments)
	r.POST("/checktxs", API_CheckTXs)
	r.POST("/parsetokenid", API_ParseTokenID)
	// r.POST("/gettxsbyreceiver", API_GetTxsByReceiver)
	// }

	if serviceCfg.Mode == INDEXERMODE && serviceCfg.IndexerBucketID == 0 {
		r.POST("/submitotakey", API_SubmitOTA)
	}
	r.Run("0.0.0.0:" + strconv.Itoa(serviceCfg.APIPort))
}

func API_GetCoins(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	viewkey := c.Query("viewkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")
	log.Println("tokenid", tokenid, common.PRVCoinID.String())
	if tokenid == "" {
		tokenid = common.PRVCoinID.String()
	}
	if version != 1 && version != 2 {
		version = 1
	}
	var result []interface{}
	var pubkey string
	highestIdx := uint64(0)
	if version == 2 {
		if otakey != "" {
			wl, err := wallet.Base58CheckDeserialize(otakey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid otakey")))
				return
			}
			pukeyBytes := wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
			pubkey = base58.EncodeCheck(pukeyBytes)
			shardID := common.GetShardIDFromLastByte(pukeyBytes[len(pukeyBytes)-1])
			tokenidv2 := tokenid
			// if tokenid != common.PRVCoinID.String() && tokenid != common.ConfidentialAssetID.String() {
			// 	tokenidv2 = common.ConfidentialAssetID.String()
			// }
			coinList, err := DBGetCoinsByOTAKey(int(shardID), tokenidv2, base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), int64(offset), int64(limit))
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}

			for _, cn := range coinList {
				if cn.CoinIndex > highestIdx {
					highestIdx = cn.CoinIndex
				}
				coinV2 := new(coin.CoinV2)
				err := coinV2.SetBytes(cn.Coin)
				if err != nil {
					c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
					return
				}
				idx := new(big.Int).SetUint64(cn.CoinIndex)
				cV2 := jsonresult.NewOutCoin(coinV2)
				cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
				result = append(result, cV2)
			}
		}
	}
	if version == 1 {
		var viewKeySet *incognitokey.KeySet
		if viewkey == "" {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("viewkey cant be empty")))
			return
		}
		wl, err := wallet.Base58CheckDeserialize(viewkey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if wl.KeySet.ReadonlyKey.Rk == nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid viewkey")))
			return
		}
		pubkey = base58.EncodeCheck(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		wl.KeySet.PaymentAddress.Pk = wl.KeySet.ReadonlyKey.Pk
		viewKeySet = &wl.KeySet
		coinListV1, err := DBGetCoinV1ByPubkey(tokenid, hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS()), int64(offset), int64(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}
		var wg sync.WaitGroup
		collectCh := make(chan OutCoinV1, MAX_CONCURRENT_COIN_DECRYPT)
		decryptCount := 0
		for idx, cdata := range coinListV1 {
			if cdata.CoinIndex > highestIdx {
				highestIdx = cdata.CoinIndex
			}
			coinV1 := new(coin.CoinV1)
			err := coinV1.SetBytes(cdata.Coin)
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			if viewKeySet != nil {
				wg.Add(1)
				go func(cn *coin.CoinV1, cData CoinData) {
					plainCoin, err := cn.Decrypt(viewKeySet)
					if err != nil {
						c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
						return
					}
					cV1 := NewOutCoinV1(plainCoin)
					idx := new(big.Int).SetUint64(cData.CoinIndex)
					cV1.Index = base58.EncodeCheck(idx.Bytes())
					collectCh <- cV1
					wg.Done()
				}(coinV1, cdata)
				if decryptCount%MAX_CONCURRENT_COIN_DECRYPT == 0 || idx+1 == len(coinListV1) {
					wg.Wait()
					close(collectCh)
					for coin := range collectCh {
						result = append(result, coin)
					}
					collectCh = make(chan OutCoinV1, MAX_CONCURRENT_COIN_DECRYPT)
				}
			} else {
				cn := NewOutCoinV1(coinV1)
				result = append(result, cn)
			}
		}
	}

	rs := make(map[string]interface{})
	rs["HighestIndex"] = highestIdx
	rs["Outputs"] = map[string]interface{}{pubkey: result}
	respond := API_respond{
		Result: rs,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func API_GetKeyInfo(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	if version != 1 && version != 2 {
		version = 1
	}
	key := c.Query("key")
	if key != "" {
		if version == 1 {
			wl, err := wallet.Base58CheckDeserialize(key)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			pubkey := hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
			result, err := DBGetCoinV1PubkeyInfo(pubkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			respond := API_respond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		} else {
			wl, err := wallet.Base58CheckDeserialize(key)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			otakey := hex.EncodeToString(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
			result, err := DBGetCoinV2PubkeyInfo(otakey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			respond := API_respond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		}
	} else {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("key cant be empty")))
		return
	}
}

func API_CheckKeyImages(c *gin.Context) {
	var req API_check_keyimages_request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	result, err := DBCheckKeyimagesUsed(req.Keyimages, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func API_GetRandomCommitments(c *gin.Context) {

	var req API_get_random_commitment_request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	commitmentIndices := []uint64{}
	myIndices := []uint64{}
	var publicKeys, commitments, assetTags []string

	if req.Version == 1 && len(req.Indexes) > 0 {
		result, err := DBGetCoinV1ByIndexes(req.Indexes, req.ShardID, req.TokenID)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if len(req.Indexes) != len(result) {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(req.Indexes) != len(result)")))
			return
		}
		listUsableCommitments := make(map[common.Hash][]byte)
		listUsableCommitmentsIndices := make([]common.Hash, len(req.Indexes))
		mapIndexCommitmentsInUsableTx := make(map[string]*big.Int)
		usableInputCoins := []*coin.CoinV1{}
		for i, c := range result {
			coinV1 := new(coin.CoinV1)
			coinV1.SetBytes(c.Coin)
			usableInputCoins = append(usableInputCoins, coinV1)
			usableCommitment := coinV1.GetCommitment().ToBytesS()
			commitmentInHash := common.HashH(usableCommitment)
			listUsableCommitments[commitmentInHash] = usableCommitment
			listUsableCommitmentsIndices[i] = commitmentInHash
			commitmentInBase58Check := base58.Base58Check{}.Encode(usableCommitment, common.ZeroByte)
			mapIndexCommitmentsInUsableTx[commitmentInBase58Check] = new(big.Int).SetUint64(c.CoinIndex)
		}

		// loop to random commitmentIndexs
		cpRandNum := (len(listUsableCommitments) * privacy.CommitmentRingSize) - len(listUsableCommitments)
		//fmt.Printf("cpRandNum: %d\n", cpRandNum)
		lenCommitment := new(big.Int).SetInt64(DBGetCoinV1OfShardCount(req.ShardID, req.TokenID))
		if lenCommitment == nil {
			log.Println(errors.New("Commitments is empty"))
			return
		}
		if lenCommitment.Uint64() == 1 && len(req.Indexes) == 1 {
			commitmentIndices = []uint64{0, 0, 0, 0, 0, 0, 0}
			temp := base58.EncodeCheck(usableInputCoins[0].GetCommitment().ToBytesS())
			commitments = []string{temp, temp, temp, temp, temp, temp, temp}
		} else {
			randIdxs := []uint64{}
			for i := 0; i < cpRandNum; i++ {
			random:
				index, _ := common.RandBigIntMaxRange(lenCommitment)
				for _, v := range mapIndexCommitmentsInUsableTx {
					if index.Uint64() == v.Uint64() {
						goto random
					}
				}
				randIdxs = append(randIdxs, index.Uint64())
			}

			coinList, err := DBGetCoinV1ByIndexes(randIdxs, req.ShardID, req.TokenID)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if len(randIdxs) != len(coinList) {
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(randIdxs) != len(coinList)")))
					return
				}
			}
			for _, c := range coinList {
				coinV1 := new(coin.CoinV1)
				coinV1.SetBytes(c.Coin)
				commitmentIndices = append(commitmentIndices, c.CoinIndex)
				commitments = append(commitments, base58.EncodeCheck(coinV1.GetCommitment().ToBytesS()))
			}
		}
		// loop to insert usable commitments into commitmentIndexs for every group
		j := 0
		for _, commitmentInHash := range listUsableCommitmentsIndices {
			commitmentValue := base58.Base58Check{}.Encode(listUsableCommitments[commitmentInHash], common.ZeroByte)
			index := mapIndexCommitmentsInUsableTx[commitmentValue]
			randInt := rand.Intn(privacy.CommitmentRingSize)
			i := (j * privacy.CommitmentRingSize) + randInt
			commitmentIndices = append(commitmentIndices[:i], append([]uint64{index.Uint64()}, commitmentIndices[i:]...)...)
			commitments = append(commitments[:i], append([]string{commitmentValue}, commitments[i:]...)...)
			myIndices = append(myIndices, uint64(i)) // create myCommitmentIndexs
			j++
		}

	}
	if req.Version == 2 && req.Limit > 0 {
		if req.TokenID != common.PRVCoinID.String() {
			req.TokenID = common.ConfidentialAssetID.String()
		}
		lenOTA := new(big.Int).SetInt64(DBGetCoinV2OfShardCount(req.ShardID, req.TokenID) - 1)
		var hasAssetTags bool = true
		for i := 0; i < req.Limit; i++ {
			idx, _ := common.RandBigIntMaxRange(lenOTA)
			log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
			coinData, err := DBGetCoinsByIndex(int(idx.Int64()), req.ShardID, req.TokenID)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			coinV2 := new(coin.CoinV2)
			err = coinV2.SetBytes(coinData.Coin)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			publicKey := coinV2.GetPublicKey()
			commitment := coinV2.GetCommitment()

			commitmentIndices = append(commitmentIndices, idx.Uint64())
			publicKeys = append(publicKeys, base58.EncodeCheck(publicKey.ToBytesS()))
			commitments = append(commitments, base58.EncodeCheck(commitment.ToBytesS()))

			if hasAssetTags {
				assetTag := coinV2.GetAssetTag()
				if assetTag != nil {
					assetTags = append(assetTags, base58.EncodeCheck(assetTag.ToBytesS()))
				} else {
					hasAssetTags = false
				}
			}
		}

	}

	rs := make(map[string]interface{})
	rs["CommitmentIndices"] = commitmentIndices
	rs["MyIndices"] = myIndices
	rs["PublicKeys"] = publicKeys
	rs["Commitments"] = commitments
	rs["AssetTags"] = assetTags
	respond := API_respond{
		Result: rs,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func API_GetCoinsPending(c *gin.Context) {
	result, err := DBGetPendingCoins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func API_SubmitOTA(c *gin.Context) {
	var req API_submit_otakey_request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	wl, err := wallet.Base58CheckDeserialize(req.OTAKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("OTASecretKey is invalid")))
		return
	}
	otaKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
	pubKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())

	newSubmitRequest := NewSubmittedOTAKeyData(otaKey, pubKey, 0)
	resp := make(chan error)
	otaAssignChn <- otaAssignRequest{
		Key:     newSubmitRequest,
		Respond: resp,
	}
	err = <-resp
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func API_CheckTXs(c *gin.Context) {
	var req API_check_tx_request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	result, err := DBCheckTxsExist(req.Txs, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

// func API_GetTxsByReceiver(c *gin.Context) {
// var req API_get_txs_request
// err := c.ShouldBindJSON(&req)
// if err != nil {
// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
// 	return
// }

// result, err := DBCheckTxsExist(req.Txs, req.ShardID)
// if err != nil {
// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
// 	return
// }

// respond := API_respond{
// 	Result: result,
// 	Error:  nil,
// }
// c.JSON(http.StatusOK, respond)
// }

func API_ParseTokenID(c *gin.Context) {
	var req API_parse_tokenid_request
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if len(req.AssetTags) != len(req.OTARandoms) {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(req.AssetTags) != len(req.ShardSecrets)")))
		return
	}
	assetTags, err := AssetTagStringToPoint(req.AssetTags)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	sharedSecrets, err := CalculateSharedSecret(req.OTARandoms, req.OTAKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	tokenCount, err := DBGetTokenCount()
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	tokenIDs := []*common.Hash{}
	tokenIDstrs := []string{}
	if tokenCount != lastTokenListCount {
		tokenInfos, err := DBGetTokenInfo()
		if err != nil {
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}
		for _, tokenInfo := range tokenInfos {
			tokenID, err := new(common.Hash).NewHashFromStr(tokenInfo.TokenID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			tokenIDs = append(tokenIDs, tokenID)
			tokenIDstrs = append(tokenIDstrs, tokenInfo.TokenID)
		}
		tokenListLock.Lock()
		lastTokenListCount = tokenCount
		lastTokenIDHash = tokenIDs
		lastTokenIDstr = tokenIDstrs
		tokenListLock.Unlock()
	} else {
		tokenListLock.Lock()
		copy(tokenIDs, lastTokenIDHash)
		copy(tokenIDstrs, lastTokenIDstr)
		tokenListLock.Unlock()
	}
	var wg sync.WaitGroup
	tempIDCheckCh := make(chan map[int]int, 10)
	result := make([]string, len(sharedSecrets))
	for idx, sharedSecret := range sharedSecrets {
		wg.Add(1)
		go func(i int, sc *operation.Point) {
			var ok bool
			var tokenIDidx int
			for ti, tokenID := range tokenIDs {
				if ok, _ = CheckTokenIDWithOTA(sc, assetTags[i], tokenID); ok {
					tokenIDidx = ti
					break
				}
			}
			tempIDCheckCh <- map[int]int{i: tokenIDidx}
			wg.Done()
		}(idx, sharedSecret)

		if (idx+1)%serviceCfg.MaxConcurrentOTACheck == 0 || idx+1 == len(sharedSecrets) {
			wg.Wait()
			close(tempIDCheckCh)
			for k := range tempIDCheckCh {
				for coinIdx, tokenIdx := range k {
					result[coinIdx] = tokenIDstrs[tokenIdx]
				}
			}
			if idx+1 != len(sharedSecrets) {
				tempIDCheckCh = make(chan map[int]int, 10)
			}
		}
	}
	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func API_HealthCheck(c *gin.Context) {
	//check block time
	//ping pong vs mongo
	status := "healthy"
	mongoStatus := "connected"
	shardsHeight := make(map[int]string)
	if serviceCfg.Mode == CHAINSYNCMODE {
		now := time.Now().Unix()
		blockTime := localnode.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetProposeTime()
		if (now - blockTime) >= (5 * time.Minute).Nanoseconds() {
			status = "unhealthy"
		}
		shardsHeight[-1] = fmt.Sprintf("%v", localnode.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetHeight())
		for i := 0; i < localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
			chainheight := localnode.GetBlockchain().BeaconChain.GetShardBestViewHeight()[byte(i)]
			height, _ := localnode.GetShardState(i)
			statePrefix := fmt.Sprintf("coin-processed-%v", i)
			v, err := localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
			if err != nil {
				log.Println(err)
			}
			coinHeight, err := strconv.Atoi(string(v))
			if err != nil {
				coinHeight = 0
			}
			if chainheight-height > 5 || height-uint64(coinHeight) > 5 {
				status = "unhealthy"
			}
			shardsHeight[i] = fmt.Sprintf("%v|%v|%v", coinHeight, height, chainheight)
		}
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err := cd.Ping(context.Background(), nil)
	if err != nil {
		status = "unhealthy"
		mongoStatus = "disconnected"
	}
	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"mongo":  mongoStatus,
		"chain":  shardsHeight,
	})
}

func buildGinErrorRespond(err error) *API_respond {
	errStr := err.Error()
	respond := API_respond{
		Result: nil,
		Error:  &errStr,
	}
	return &respond
}
