package apiservice

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/shared"
	jsoniter "github.com/json-iterator/go"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy/coin"
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

	if shared.ServiceCfg.Mode == shared.QUERYMODE || shared.ServiceCfg.Mode == shared.FULLMODE {
		r.GET("/getcoinslength", APIGetCoinInfo)
		r.GET("/getcoinspending", APIGetCoinsPending)
		r.GET("/getcoins", APIGetCoins)
		r.GET("/getkeyinfo", APIGetKeyInfo)
		r.POST("/checkkeyimages", APICheckKeyImages)
		r.POST("/getrandomcommitments", APIGetRandomCommitments)
		r.POST("/checktxs", APICheckTXs)
		r.GET("/gettxdetail", APIGetTxDetail)
		r.POST("/gettxsbypubkey", APIGetTxsByPubkey)
		r.GET("/getpendingtxs", APIGetPendingTxs)
		r.GET("/checkpendingtx", APICheckTxPending)
		r.GET("/tokenlist", APIGetTokenList)

		r.GET("/gettxsbyreceiver", APIGetTxsByReceiver)
		r.POST("/gettxsbysender", APIGetTxsBySender)

		r.GET("/getlatesttx", APIGetLatestTxs)
		r.GET("/gettradehistory", APIGetTradeHistory)
		r.GET("/getpdestate", APIPDEState)
		r.GET("/getshieldhistory", APIGetShieldHistory)
		r.GET("/getunshieldhistory", APIGetUnshieldHistory)
		r.GET("/getcontributehistory", APIGetContributeHistory)
		r.GET("/getwithdrawhistory", APIGetWithdrawHistory)
		r.GET("/getwithdrawfeehistory", APIGetWithdrawFeeHistory)

		// New API format
		//coins
		coinsGroup := r.Group("/coins")
		coinsGroup.GET("/tokenlist", APIGetTokenList)
		coinsGroup.GET("/getcoinslength", APIGetCoinInfo)
		coinsGroup.GET("/getcoinspending", APIGetCoinsPending)
		coinsGroup.GET("/getcoins", APIGetCoins)
		coinsGroup.GET("/getkeyinfo", APIGetKeyInfo)
		coinsGroup.POST("/checkkeyimages", APICheckKeyImages)
		coinsGroup.POST("/getrandomcommitments", APIGetRandomCommitments)
		//tx
		txGroup := r.Group("/txs")
		txGroup.POST("/gettxsbysender", APIGetTxsBySender)
		txGroup.POST("/checktxs", APICheckTXs)
		txGroup.POST("/gettxsbypubkey", APIGetTxsByPubkey)
		txGroup.GET("/gettxsbyreceiver", APIGetTxsByReceiver)
		txGroup.GET("/gettxdetail", APIGetTxDetail)
		txGroup.GET("/getpendingtxs", APIGetPendingTxs)
		txGroup.GET("/checkpendingtx", APICheckTxPending)
		txGroup.GET("/getlatesttx", APIGetLatestTxs)
		//shield
		shieldGroup := r.Group("/shield")
		shieldGroup.GET("/getshieldhistory", APIGetShieldHistory)
		shieldGroup.GET("/getunshieldhistory", APIGetUnshieldHistory)
	}

	if shared.ServiceCfg.Mode == shared.INDEXERMODE {
		r.POST("/submitotakey", APISubmitOTA)
		r.POST("/rescanotakey", APIRescanOTA)
		r.GET("/indexworker", otaindexer.WorkerRegisterHandler)
		r.GET("/workerstat", otaindexer.GetWorkerStat)
	}

	if shared.ServiceCfg.Mode == shared.FULLMODE {
		r.POST("/submitotakey", APISubmitOTAFullmode)
	}
	err := r.Run("0.0.0.0:" + strconv.Itoa(shared.ServiceCfg.APIPort))
	if err != nil {
		panic(err)
	}
}

func APIGetCoins(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")
	viewkey := c.Query("viewkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")
	shardid, _ := strconv.Atoi(c.Query("shardid"))

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
				cV2.TxHash = cn.TxHash
				result = append(result, cV2)
			}
		} else {
			coinList, err := database.DBGetUnknownCoinsV21(shardid, tokenid, int64(offset), int64(limit))
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
				cV2.TxHash = cn.TxHash
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
					cV1.TxHash = cData.TxHash
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
				cV1.TxHash = cdata.TxHash
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

func APIRescanOTA(c *gin.Context) {
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

	err = otaindexer.ReScanOTAKey(otaKey, pubKey)
	respond := APIRespond{
		Result: "true",
	}
	if err != nil {
		errStr := err.Error()
		respond = APIRespond{
			Result: "false",
			Error:  &errStr,
		}
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

	newSubmitRequest := shared.NewSubmittedOTAKeyData(otaKey, pubKey, req.OTAKey, 0)
	resp := make(chan error)
	otaindexer.OTAAssignChn <- otaindexer.OTAAssignRequest{
		Key:     newSubmitRequest,
		Respond: resp,
	}
	err = <-resp
	errStr := ""
	if err != nil {
		// if !mongo.IsDuplicateKeyError(err) {
		// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		// 	return
		// }
		errStr = err.Error()
	}
	respond := APIRespond{
		Result: "true",
		Error:  &errStr,
	}
	c.JSON(http.StatusOK, respond)
}

func APISubmitOTAFullmode(c *gin.Context) {
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

	newSubmitRequest := shared.NewSubmittedOTAKeyData(otaKey, pubKey, req.OTAKey, 0)
	resp := make(chan error)
	otaindexer.OTAAssignChn <- otaindexer.OTAAssignRequest{
		Key:     newSubmitRequest,
		Respond: resp,
	}
	err = <-resp
	errStr := ""
	if err != nil {
		// if !mongo.IsDuplicateKeyError(err) {
		// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		// 	return
		// }
		errStr = err.Error()
	}
	respond := APIRespond{
		Result: "true",
		Error:  &errStr,
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
		for i := 0; i < chainsynker.Localnode.GetBlockchain().GetActiveShardNumber(); i++ {
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

func buildGinErrorRespond(err error) *APIRespond {
	errStr := err.Error()
	respond := APIRespond{
		Result: nil,
		Error:  &errStr,
	}
	return &respond
}

func buildTxDetailRespond(txDataList []shared.TxData, isBase58 bool) ([]ReceivedTransactionV2, error) {
	var wg sync.WaitGroup
	collectCh := make(chan ReceivedTransactionV2, 200)
	var result []ReceivedTransactionV2
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
			txDetail.PrivacyCustomTokenID = txd.RealTokenID
			txReceive := ReceivedTransactionV2{
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
			collectCh = make(chan ReceivedTransactionV2, 200)
		}
	}
	return result, errD
}
