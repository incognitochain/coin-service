package apiservice

import (
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/wallet"
)

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
func APIGetTxsByPubkey(c *gin.Context) {
	var req APIGetTxsByPubkeyRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if len(req.Pubkeys) == 0 {
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("len(req.Pubkeys) == 0")))
			return
		}
	}
	if !req.Base58 {
		newList := []string{}
		for _, v := range req.Pubkeys {
			d, _ := base64.StdEncoding.DecodeString(v)
			s := base58.EncodeCheck(d)
			newList = append(newList, s)
		}
		req.Pubkeys = newList
	}
	txDataList, txsPubkey, err := database.DBGetTxV2ByPubkey(req.Pubkeys)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	txDetailList, err := buildTxDetailRespond(txDataList, req.Base58)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	result := []*ReceivedTransactionV2{}
	for _, txPubkey := range txsPubkey {
		var tx *ReceivedTransactionV2
		if txPubkey != "" {
			for _, txDetail := range txDetailList {
				if txDetail.TxDetail.Hash == txPubkey {
					tx = &txDetail
					break
				}
			}
		}
		result = append(result, tx)
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
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
	if !req.Base58 {
		newList := []string{}
		for _, v := range req.Keyimages {
			d, _ := base64.StdEncoding.DecodeString(v)
			s := base58.EncodeCheck(d)
			newList = append(newList, s)
		}
		req.Keyimages = newList
	}
	txDataList, err := database.DBGetSendTxByKeyImages(req.Keyimages)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	result := []*ReceivedTransactionV2{}
	matchTxList := matchTxDataWithKeyimage(req.Keyimages, txDataList)

	txDetailList, err := buildTxDetailRespond(txDataList, req.Base58)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	for _, txMatch := range matchTxList {
		var tx *ReceivedTransactionV2
		if txMatch != nil {
			for _, txDetail := range txDetailList {
				if txDetail.TxDetail.Hash == txMatch.TxHash {
					tx = &txDetail
					break
				}
			}
		}
		result = append(result, tx)
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	log.Println("APIGetTxsBySender time:", time.Since(startTime))
	c.JSON(http.StatusOK, respond)
}

func matchTxDataWithKeyimage(keyImages []string, txList []shared.TxData) []*shared.TxData {
	var result []*shared.TxData
	for _, km := range keyImages {
		var matchTx *shared.TxData
		for _, tx := range txList {
			txkm := strings.Join(tx.KeyImages, ",")
			if strings.Contains(txkm, km) {
				matchTx = &tx
				break
			}
		}
		result = append(result, matchTx)
	}
	return result
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
	shardID, _ := strconv.Atoi(c.Query("shardid"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	isBase58 := false
	if c.Query("base58") == "true" {
		isBase58 = true
	}
	txDataList, err := database.DBGetLatestTxByShardID(shardID, int64(limit))
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
	c.JSON(http.StatusOK, respond)
}

func APIGetTxDetail(c *gin.Context) {
	txhash := c.Query("txhash")
	if txhash == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid txhash")))
		return
	}
	isBase58 := false
	if c.Query("base58") == "true" {
		isBase58 = true
	}
	returnRaw := false
	if c.Query("raw") == "true" {
		returnRaw = true
	}
	txDataList, err := database.DBGetTxByHash([]string{txhash})
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if !returnRaw {
		result, err := buildTxDetailRespond(txDataList, isBase58)
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
		result := []string{}
		for _, v := range txDataList {
			result = append(result, v.TxDetail)
		}
		respond := APIRespond{
			Result: result,
			Error:  nil,
		}
		c.JSON(http.StatusOK, respond)
	}

}
