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
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/kamva/mgm/v3"
)

func startGinService() {
	log.Println("initiating api-service...")
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/health", API_HealthCheck)
	r.GET("/getcoinspending", API_GetCoinsPending)
	r.GET("/getcoins", API_GetCoins)
	r.GET("/getkeyinfo", API_GetKeyInfo)
	r.POST("/checkkeyimages", API_CheckKeyImages)
	r.POST("/getrandomcommitments", API_GetRandomCommitments)
	r.POST("/submitotakey", API_SubmitOTA)
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
			pubkey = base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
			tokenidv2 := tokenid
			if tokenid != common.PRVCoinID.String() && tokenid != common.ConfidentialAssetID.String() {
				tokenidv2 = common.ConfidentialAssetID.String()
			}
			coinList, err := DBGetCoinsByOTAKeyAndHeight(tokenidv2, hex.EncodeToString(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), offset, limit)
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
		// fmt.Println(wl.Base58CheckSerialize(wallet.PaymentAddressType))
		// fmt.Println(wl.Base58CheckSerialize(wallet.ReadonlyKeyType))
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
	key := c.Query("key")
	if key != "" {
		wl, err := wallet.Base58CheckDeserialize(key)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("key cant be empty")))
			return
		}
		pubkey := hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		result, err := DBGetCoinPubkeyInfo(pubkey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("key cant be empty")))
			return
		}
		respond := API_respond{
			Result: result,
			Error:  nil,
		}
		c.JSON(http.StatusOK, respond)
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
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			if len(randIdxs) != len(coinList) {
				if err != nil {
					c.JSON(http.StatusInternalServerError, buildGinErrorRespond(errors.New("len(randIdxs) != len(coinList)")))
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
	err = addOTAKey(req.OTAKey, 0, 0, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
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
			height, _ := localnode.GetShardState(i)
			chainheight := localnode.GetBlockchain().BeaconChain.GetShardBestViewHeight()[byte(i)]
			if chainheight-height > 6 {
				status = "unhealthy"
			}
			shardsHeight[i] = fmt.Sprintf("%v/%v", height, chainheight)
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
