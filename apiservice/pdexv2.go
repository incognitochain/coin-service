package apiservice

import (
	"errors"
	"log"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

func APIGetTradeHistory(c *gin.Context) {
	startTime := time.Now()
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	otakey := c.Query("otakey")
	paymentkey := c.Query("paymentkey")
	key := ""
	// pubKeyStr := ""
	// pubKeyBytes := []byte{}
	// if otakey != "" {
	// 	wl, err := wallet.Base58CheckDeserialize(otakey)
	// 	if err != nil {
	// 		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 		return
	// 	}
	// 	pubKeyBytes = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	// } else {
	// 	if paymentkey == "" {
	// 		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
	// 		return
	// 	}
	// 	wl, err := wallet.Base58CheckDeserialize(paymentkey)
	// 	if err != nil {
	// 		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 		return
	// 	}
	// 	pubKeyBytes = wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	// }
	// pubKeyStr = base58.EncodeCheck(pubKeyBytes)
	if otakey != "" {
		key = otakey
	}
	if paymentkey != "" {
		key = paymentkey
	}
	pubKeyStr, err := extractPubkeyFromKey(key, false)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

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

	txTradePairlist, err := database.DBGetTxTradeFromTxRespond(respList)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var result []*TxTradeDetail
	txsMap := make(map[string]int)
	txsRequest := []string{}
	for _, v := range txTradePairlist {
		if idx, ok := txsMap[v.RequestTx]; !ok {
			statusStr := ""
			switch v.Status {
			case 0:
				statusStr = "pending"
			case 1:
				statusStr = "accepted"
			case 2:
				statusStr = "rejected"
			}
			newTxDetail := TxTradeDetail{
				RequestTx:     v.RequestTx,
				RespondTx:     v.RespondTxs,
				Status:        statusStr,
				ReceiveAmount: make(map[string]uint64),
			}
			newTxDetail.ReceiveAmount[v.BuyTokenID] = v.Amount
			txsMap[v.RequestTx] = len(result)
			result = append(result, &newTxDetail)
			txsRequest = append(txsRequest, v.RequestTx)
		} else {
			txdetail := result[idx]
			txdetail.RespondTx = append(txdetail.RespondTx, v.RespondTxs...)
			txdetail.ReceiveAmount[v.BuyTokenID] = v.Amount
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

func APIGetWithdrawHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")

	paymentkeyOld, err := wallet.GetPaymentAddressV1(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	contrData, err := database.DBGetPDEWithdrawRespond([]string{paymentkey, paymentkeyOld}, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	type DataWithLockTime struct {
		shared.WithdrawContributionData
		Locktime int64
	}

	var result []DataWithLockTime
	for _, contr := range contrData {
		tx, err := database.DBGetTxByHash([]string{contr.RequestTx})
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		result = append(result, DataWithLockTime{
			contr, tx[0].Locktime,
		})
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetWithdrawFeeHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")

	paymentkeyOld, err := wallet.GetPaymentAddressV1(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	contrData, err := database.DBGetPDEWithdrawFeeRespond([]string{paymentkeyOld, paymentkey}, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	type DataWithLockTime struct {
		shared.WithdrawContributionFeeData
		Locktime int64
	}

	var result []DataWithLockTime
	for _, contr := range contrData {
		tx, err := database.DBGetTxByHash([]string{contr.RequestTx})
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		result = append(result, DataWithLockTime{
			contr, tx[0].Locktime,
		})
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetContributeHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")

	paymentkeyOld, err := wallet.GetPaymentAddressV1(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	contrData, err := database.DBGetPDEContributeRespond([]string{paymentkeyOld, paymentkey}, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	contrDataNoDup := checkDup(contrData)
	type DataWithLockTime struct {
		shared.ContributionData
		Locktime int64
	}
	var result []DataWithLockTime
	for _, contr := range contrDataNoDup {
		tx, err := database.DBGetTxByHash(contr.RequestTxs)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		result = append(result, DataWithLockTime{
			contr, tx[0].Locktime,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Locktime > result[j].Locktime
	})
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func checkDup(list []shared.ContributionData) []shared.ContributionData {
	checkVal := make(map[string]int)
	newList := []shared.ContributionData{}
	for _, v := range list {
		if _, ok := checkVal[v.ID.Hex()]; !ok {
			checkVal[v.ID.Hex()] = 1
			newList = append(newList, v)
		}
	}
	return newList
}
