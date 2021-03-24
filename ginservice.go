package main

import (
	"encoding/hex"
	"errors"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

func startGinService() {
	log.Println("initiating api-service...")
	r := gin.Default()
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
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func API_GetCoins(c *gin.Context) {
	version, _ := strconv.Atoi(c.Query("version"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	viewkey := c.Query("viewkey")
	otakey := c.Query("otakey")
	tokenid := c.Query("tokenid")

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

}

func API_CheckKeyImages(c *gin.Context) {

}

func API_GetRandomCommitments(c *gin.Context) {

}

func API_GetCoinsPending(c *gin.Context) {

}

func API_SubmitOTA(c *gin.Context) {

}

func API_HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
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
