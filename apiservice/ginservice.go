package apiservice

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/shared"
	jsoniter "github.com/json-iterator/go"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/kamva/mgm/v3"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func StartGinService() {
	log.Println("initiating api-service...")

	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.GET("/health", APIHealthCheck)
	// if serviceCfg.Mode == QUERYMODE {
	r.GET("/getcoininfo", APIGetCoinInfo)
	r.GET("/getcoinspending", APIGetCoinsPending)
	r.GET("/getcoins", APIGetCoins)
	r.GET("/getkeyinfo", APIGetKeyInfo)
	r.POST("/checkkeyimages", APICheckKeyImages)
	r.POST("/getrandomcommitments", APIGetRandomCommitments)
	r.POST("/checktxs", APICheckTXs)

	r.GET("/gettxsbyreceiver", APIGetTxsByReceiver)
	r.POST("/gettxsbysender", APIGetTxsBySender)

	r.GET("/getlatesttx", APIGetLatestTxs)
	r.GET("/gettradehistory", APIGetTradeHistory)
	r.GET("/getpdestate", APIPDEState)
	// }

	if shared.ServiceCfg.Mode == shared.INDEXERMODE && shared.ServiceCfg.IndexerBucketID == 0 {
		r.POST("/submitotakey", APISubmitOTA)
	}
	r.Run("0.0.0.0:" + strconv.Itoa(shared.ServiceCfg.APIPort))
}

func APIGetCoins(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")
	viewkey := c.Query("viewkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")
	base58Format := false
	log.Println("tokenid", tokenid, common.PRVCoinID.String())
	if tokenid == "" {
		tokenid = common.PRVCoinID.String()
	}
	if c.Query("base58") == "true" {
		base58Format = true
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
			coinList, err := database.DBGetCoinsByOTAKey(int(shardID), tokenidv2, base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), int64(offset), int64(limit))
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
				var cV2 shared.OutCoinV2
				if base58Format {
					cV2 = shared.NewOutCoinV2(coinV2, true)
					cV2.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
				} else {
					cV2 = shared.NewOutCoinV2(coinV2, false)
					cV2.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
				}

				result = append(result, cV2)
			}
		}
	}
	if version == 1 {
		var viewKeySet *incognitokey.KeySet
		if viewkey != "" {
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
		} else {
			if paymentkey == "" {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("paymentkey cant be empty")))
				return
			}
			wl, err := wallet.Base58CheckDeserialize(paymentkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS())
		}

		coinListV1, err := database.DBGetCoinV1ByPubkey(tokenid, pubkey, int64(offset), int64(limit))
		if err != nil {
			c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
			return
		}
		var wg sync.WaitGroup
		collectCh := make(chan shared.OutCoinV1, shared.MAX_CONCURRENT_COIN_DECRYPT)
		// decryptCount := 0
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
				go func(cn *coin.CoinV1, cData shared.CoinData) {
					plainCoin, err := cn.Decrypt(viewKeySet)
					if err != nil {
						c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
						return
					}
					// cV1 := shared.NewOutCoinV1(plainCoin)

					idx := new(big.Int).SetUint64(cData.CoinIndex)
					var cV1 shared.OutCoinV1
					if base58Format {
						cV1 = shared.NewOutCoinV1(plainCoin, true)
						cV1.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
					} else {
						cV1 = shared.NewOutCoinV1(plainCoin, false)
						cV1.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
					}
					collectCh <- cV1
					wg.Done()
				}(coinV1, cdata)
				if (idx+1)%shared.MAX_CONCURRENT_COIN_DECRYPT == 0 || idx+1 == len(coinListV1) {
					wg.Wait()
					close(collectCh)
					for coin := range collectCh {
						result = append(result, coin)
					}
					collectCh = make(chan shared.OutCoinV1, shared.MAX_CONCURRENT_COIN_DECRYPT)
				}
			} else {

				idx := new(big.Int).SetUint64(cdata.CoinIndex)
				var cV1 shared.OutCoinV1
				if base58Format {
					cV1 = shared.NewOutCoinV1(coinV1, true)
					cV1.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
					cV1.CoinDetailsEncrypted = base58.Base58Check{}.Encode(coinV1.GetCoinDetailEncrypted(), common.ZeroByte)
				} else {
					cV1 = shared.NewOutCoinV1(coinV1, false)
					cV1.Index = base64.StdEncoding.EncodeToString(idx.Bytes())
					cV1.CoinDetailsEncrypted = base64.StdEncoding.EncodeToString(coinV1.GetCoinDetailEncrypted())
				}
				result = append(result, cV1)
			}
		}
	}

	rs := make(map[string]interface{})
	rs["HighestIndex"] = highestIdx
	rs["Outputs"] = map[string]interface{}{pubkey: result}
	respond := APIRespond{
		Result: rs,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetKeyInfo(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	if version != 1 && version != 2 {
		version = 1
	}
	key := c.Query("key")
	if key != "" {
		wl, err := wallet.Base58CheckDeserialize(key)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}

		pubkey := ""
		if wl.KeySet.OTAKey.GetPublicSpend() != nil {
			pubkey = base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
		}
		if wl.KeySet.ReadonlyKey.GetPublicSpend() != nil && pubkey == "" {
			pubkey = base58.EncodeCheck(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		}
		if wl.KeySet.PaymentAddress.GetPublicSpend() != nil && pubkey == "" {
			pubkey = base58.EncodeCheck(wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS())
		}
		if version == 1 {
			result, err := database.DBGetCoinV1PubkeyInfo(pubkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			respond := APIRespond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
		} else {
			result, err := database.DBGetCoinV2PubkeyInfo(pubkey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			delete(result.CoinIndex, common.ConfidentialAssetID.String())
			respond := APIRespond{
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

func APICheckKeyImages(c *gin.Context) {
	var req APICheckKeyImagesRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	if req.Base58 {
		newList := []string{}
		for _, v := range req.Keyimages {
			d, _ := base64.StdEncoding.DecodeString(v)
			s := base58.EncodeCheck(d)
			newList = append(newList, s)
		}
		req.Keyimages = newList
	}

	result, err := database.DBCheckKeyimagesUsed(req.Keyimages, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetRandomCommitments(c *gin.Context) {
	var req APIGetRandomCommitmentRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	commitmentIndices := []uint64{}
	myIndices := []uint64{}
	var publicKeys, commitments, assetTags []string

	if req.Version == 1 && len(req.Indexes) > 0 {
		result, err := database.DBGetCoinV1ByIndexes(req.Indexes, req.ShardID, req.TokenID)
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
			err := coinV1.SetBytes(c.Coin)
			if err != nil {
				panic(err)
			}
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
		lenCommitment := new(big.Int).SetInt64(database.DBGetCoinV1OfShardCount(req.ShardID, req.TokenID) - 1)
		if lenCommitment == nil {
			log.Println(errors.New("commitments is empty"))
			return
		}
		if lenCommitment.Uint64() == 1 && len(req.Indexes) == 1 {
			commitmentIndices = []uint64{0, 0, 0, 0, 0, 0, 0}
			temp := base58.EncodeCheck(usableInputCoins[0].GetCommitment().ToBytesS())
			commitments = []string{temp, temp, temp, temp, temp, temp, temp}
		} else {
			randIdxs := []uint64{}
			for {
				if len(randIdxs) == cpRandNum {
					break
				}
			random:
				index, _ := common.RandBigIntMaxRange(lenCommitment)
				for _, v := range mapIndexCommitmentsInUsableTx {
					if index.Uint64() == v.Uint64() {
						goto random
					}
				}
				randIdxs = append(randIdxs, index.Uint64())
			}

			coinList, err := database.DBGetCoinV1ByIndexes(randIdxs, req.ShardID, req.TokenID)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if len(randIdxs) != len(coinList) {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(randIdxs) != len(coinList)")))
				return
			}
			for _, c := range coinList {
				coinV1 := new(coin.CoinV1)
				err := coinV1.SetBytes(c.Coin)
				if err != nil {
					panic(err)
				}
				commitmentIndices = append(commitmentIndices, c.CoinIndex)
				if req.Base58 {
					commitments = append(commitments, base58.EncodeCheck(coinV1.GetCommitment().ToBytesS()))
				} else {
					commitments = append(commitments, base64.StdEncoding.EncodeToString(coinV1.GetCommitment().ToBytesS()))
				}
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
			if !req.Base58 {
				commitmentValue = base64.StdEncoding.EncodeToString(listUsableCommitments[commitmentInHash])
			}
			commitments = append(commitments[:i], append([]string{commitmentValue}, commitments[i:]...)...)
			myIndices = append(myIndices, uint64(i)) // create myCommitmentIndexs
			j++
		}
	}
	if req.Version == 2 && req.Limit > 0 {
		if req.TokenID != common.PRVCoinID.String() {
			req.TokenID = common.ConfidentialAssetID.String()
		}
		lenOTA := new(big.Int).SetInt64(database.DBGetCoinV2OfShardCount(req.ShardID, req.TokenID) - 1)
		var hasAssetTags bool = true
		for i := 0; i < req.Limit; i++ {
			idx, _ := common.RandBigIntMaxRange(lenOTA)
			log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
			coinData, err := database.DBGetCoinsByIndex(int(idx.Int64()), req.ShardID, req.TokenID)
			if err != nil {
				i--
				continue
			}
			if coinData.CoinPubkey == "1y4gnYS1Ns2K7BjQTjgfZ5nTR8JZMkMJ3CTGMj2Pk7CQkSTFgA" {
				i--
				continue
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
			if req.Base58 {
				publicKeys = append(publicKeys, base58.EncodeCheck(publicKey.ToBytesS()))
				commitments = append(commitments, base58.EncodeCheck(commitment.ToBytesS()))
			} else {
				publicKeys = append(publicKeys, base64.StdEncoding.EncodeToString(publicKey.ToBytesS()))
				commitments = append(commitments, base64.StdEncoding.EncodeToString(commitment.ToBytesS()))
			}

			if hasAssetTags {
				assetTag := coinV2.GetAssetTag()
				if assetTag != nil {
					if req.Base58 {
						assetTags = append(assetTags, base58.EncodeCheck(assetTag.ToBytesS()))
					} else {
						assetTags = append(assetTags, base64.StdEncoding.EncodeToString(assetTag.ToBytesS()))
					}
				} else {
					hasAssetTags = false
				}
			}
		}

	}

	rs := struct {
		CommitmentIndices []uint64
		MyIndices         []uint64
		PublicKeys        []string
		Commitments       []string
		AssetTags         []string
	}{
		CommitmentIndices: commitmentIndices,
		MyIndices:         myIndices,
		PublicKeys:        publicKeys,
		Commitments:       commitments,
		AssetTags:         assetTags,
	}
	respond := APIRespond{
		Result: rs,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetCoinInfo(c *gin.Context) {
	prvV1, prvV2, tokenV1, tokenV2, err := database.DBGetCoinInfo()
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	result := struct {
		TotalPRVV1   int64
		TotalPRVV2   int64
		TotalTokenV1 int64
		TotalTokenV2 int64
	}{prvV1, prvV2, tokenV1, tokenV2}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetCoinsPending(c *gin.Context) {
	base58Fmt := c.Query("base58")
	result, err := database.DBGetPendingCoins()
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	if base58Fmt == "true" {
		resultB58 := []string{}
		for _, v := range result {
			vBytes, _, _ := base58.DecodeCheck(v)
			resultB58 = append(resultB58, base64.StdEncoding.EncodeToString(vBytes))
		}
		result = resultB58
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APISubmitOTA(c *gin.Context) {
	var req APISubmitOTAkeyRequest
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

	newSubmitRequest := shared.NewSubmittedOTAKeyData(otaKey, pubKey, 0)
	resp := make(chan error)
	otaindexer.OTAAssignChn <- otaindexer.OTAAssignRequest{
		Key:     newSubmitRequest,
		Respond: resp,
	}
	err = <-resp
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	respond := APIRespond{
		Result: "true",
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APICheckTXs(c *gin.Context) {
	var req APICheckTxRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	result, err := database.DBCheckTxsExist(req.Txs, req.ShardID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIHealthCheck(c *gin.Context) {
	//check block time
	//ping pong vs mongo
	status := shared.HEALTH_STATUS_OK
	mongoStatus := shared.MONGO_STATUS_OK
	shardsHeight := make(map[int]string)
	if shared.ServiceCfg.Mode == shared.CHAINSYNCMODE {
		now := time.Now().Unix()
		blockTime := chainsynker.Localnode.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetProposeTime()
		if (now - blockTime) >= (5 * time.Minute).Nanoseconds() {
			status = shared.HEALTH_STATUS_NOK
		}
		shardsHeight[-1] = fmt.Sprintf("%v", chainsynker.Localnode.GetBlockchain().BeaconChain.GetBestView().GetBlock().GetHeight())
		for i := 0; i < chainsynker.Localnode.GetBlockchain().GetChainParams().ActiveShards; i++ {
			chainheight := chainsynker.Localnode.GetBlockchain().BeaconChain.GetShardBestViewHeight()[byte(i)]
			height, _ := chainsynker.Localnode.GetShardState(i)
			statePrefix := fmt.Sprintf("coin-processed-%v", i)
			v, err := chainsynker.Localnode.GetUserDatabase().Get([]byte(statePrefix), nil)
			if err != nil {
				log.Println(err)
			}
			coinHeight, err := strconv.Atoi(string(v))
			if err != nil {
				coinHeight = 0
			}
			if chainheight-height > 5 || height-uint64(coinHeight) > 5 {
				status = shared.HEALTH_STATUS_NOK
			}
			shardsHeight[i] = fmt.Sprintf("%v|%v|%v", coinHeight, height, chainheight)
		}
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err := cd.Ping(context.Background(), nil)
	if err != nil {
		status = shared.HEALTH_STATUS_NOK
		mongoStatus = shared.MONGO_STATUS_NOK
	}
	c.JSON(http.StatusOK, gin.H{
		"status": status,
		"mongo":  mongoStatus,
		"chain":  shardsHeight,
	})
}
func APIGetTxsBySender(c *gin.Context) {
	startTime := time.Now()
	var req APIGetTxsBySenderRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if len(req.Keyimages) == 0 {
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(req.Keyimages) == 0")))
			return
		}
	}
	txDataList, err := database.DBGetSendTxByKeyImages(req.Keyimages)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	result, err := buildTxDetailRespond(txDataList, req.Base58)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	log.Println("APIGetTxsBySender time:", time.Since(startTime))
	c.JSON(http.StatusOK, respond)
}

func APIGetTxsByReceiver(c *gin.Context) {
	paymentkey := c.Query("paymentkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")
	txversion := 1
	isBase58 := false
	if c.Query("base58") == "true" {
		isBase58 = true
	}

	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	pubKeyStr := ""
	pubKeyBytes := []byte{}
	if otakey != "" {
		txversion = 2
		wl, err := wallet.Base58CheckDeserialize(otakey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		pubKeyBytes = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	} else {
		if paymentkey == "" {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
			return
		}
		wl, err := wallet.Base58CheckDeserialize(paymentkey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		pubKeyBytes = wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	}
	pubKeyStr = base58.EncodeCheck(pubKeyBytes)
	startTime := time.Now()
	txDataList, err := database.DBGetReceiveTxByPubkey(pubKeyStr, tokenid, txversion, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	result, err := buildTxDetailRespond(txDataList, isBase58)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	log.Println("APIGetTxsByReceiver time:", time.Since(startTime))
	c.JSON(http.StatusOK, respond)
}

func APIGetLatestTxs(c *gin.Context) {

}

func APIGetTradeHistory(c *gin.Context) {
	startTime := time.Now()
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	otakey := c.Query("otakey")
	paymentkey := c.Query("paymentkey")

	pubKeyStr := ""
	pubKeyBytes := []byte{}
	if otakey != "" {
		wl, err := wallet.Base58CheckDeserialize(otakey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		pubKeyBytes = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	} else {
		if paymentkey == "" {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
			return
		}
		wl, err := wallet.Base58CheckDeserialize(paymentkey)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		pubKeyBytes = wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	}
	pubKeyStr = base58.EncodeCheck(pubKeyBytes)

	list, err := database.DBGetTxTradeRespond(pubKeyStr, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if len(list) == 0 {
		err := errors.New("len(list) == 0").Error()
		respond := APIRespond{
			Result: nil,
			Error:  &err,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

	respList := []string{}
	for _, v := range list {
		respList = append(respList, v.TxHash)
	}

	txTradePairlist, err := database.DBGetTxTrade(respList)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var result []*TxTradeDetail
	txsMap := make(map[string]int)
	txsRequest := []string{}
	for _, v := range txTradePairlist {
		if idx, ok := txsMap[v.RequestTx]; !ok {
			newTxDetail := TxTradeDetail{
				RequestTx:     v.RequestTx,
				RespondTx:     []string{v.RespondTx},
				Status:        v.Status,
				ReceiveAmount: make(map[string]uint64),
			}
			newTxDetail.ReceiveAmount[v.TokenID] = v.Amount
			txsMap[v.RequestTx] = len(result)
			result = append(result, &newTxDetail)
			txsRequest = append(txsRequest, v.RequestTx)
		} else {
			txdetail := result[idx]
			txdetail.RespondTx = append(txdetail.RespondTx, v.RespondTx)
			txdetail.ReceiveAmount[v.TokenID] = v.Amount
		}
	}

	txsReqData, err := database.DBGetTxByHash(txsRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	for idx, tx := range txsReqData {
		txDetail := result[idx]
		switch tx.Metatype {
		case strconv.Itoa(metadata.PDETradeRequestMeta):
			meta := metadata.PDETradeRequest{}
			err := json.Unmarshal([]byte(tx.Metadata), &meta)
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			txDetail.BuyToken = meta.TokenIDToBuyStr
			txDetail.SellToken = meta.TokenIDToSellStr
			txDetail.SellAmount = meta.SellAmount
			txDetail.Fee = meta.TradingFee
			txDetail.RequestTime = tx.Locktime
		case strconv.Itoa(metadata.PDECrossPoolTradeRequestMeta):
			meta := metadata.PDECrossPoolTradeRequest{}
			err := json.Unmarshal([]byte(tx.Metadata), &meta)
			if err != nil {
				c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
				return
			}
			txDetail.BuyToken = meta.TokenIDToBuyStr
			txDetail.SellToken = meta.TokenIDToSellStr
			txDetail.SellAmount = meta.SellAmount
			txDetail.Fee = meta.TradingFee
			txDetail.RequestTime = tx.Locktime
		}
	}

	reverseAny(result)

	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	log.Println("APIGetTradeHistory time:", time.Since(startTime))
	c.JSON(http.StatusOK, respond)
}

func reverseAny(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func APIPDEState(c *gin.Context) {
	state, err := database.DBGetPDEState()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	pdeState := jsonresult.CurrentPDEState{}
	err = json.UnmarshalFromString(state, &pdeState)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: pdeState,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func buildGinErrorRespond(err error) *APIRespond {
	errStr := err.Error()
	respond := APIRespond{
		Result: nil,
		Error:  &errStr,
	}
	return &respond
}

func buildTxDetailRespond(txDataList []shared.TxData, isBase58 bool) ([]jsonresult.ReceivedTransactionV2, error) {
	var wg sync.WaitGroup
	collectCh := make(chan jsonresult.ReceivedTransactionV2, 200)
	var result []jsonresult.ReceivedTransactionV2
	var errD error
	for idx, txData := range txDataList {
		wg.Add(1)
		go func(txd shared.TxData) {
			var tx metadata.Transaction
			var parseErr error
			var txChoice *transaction.TxChoice
			txChoice, parseErr = shared.DeserializeTransactionJSON([]byte(txd.TxDetail))
			if parseErr != nil {
				errD = parseErr
				return
			}
			tx = txChoice.ToTx()
			if tx == nil {
				errD = errors.New("invalid tx detected")
				return
			}
			blockHeight, err := strconv.ParseUint(txd.BlockHeight, 0, 64)
			if err != nil {
				errD = err
				return
			}
			txDetail, err := shared.NewTransactionDetail(tx, nil, blockHeight, 0, byte(txd.ShardID), isBase58)
			if err != nil {
				errD = err
				return
			}
			txDetail.BlockHash = txd.BlockHash
			txDetail.IsInBlock = true
			txDetail.Proof = nil
			txDetail.Sig = ""
			txReceive := jsonresult.ReceivedTransactionV2{
				TxDetail:    txDetail,
				FromShardID: txDetail.ShardID,
			}
			collectCh <- txReceive
			wg.Done()
		}(txData)
		if (idx+1)%200 == 0 || idx+1 == len(txDataList) {
			wg.Wait()
			close(collectCh)
			for txjson := range collectCh {
				result = append(result, txjson)
			}
			collectCh = make(chan jsonresult.ReceivedTransactionV2, 200)
		}
	}
	return result, errD
}
