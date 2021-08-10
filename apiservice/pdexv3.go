package apiservice

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/wallet"
)

type pdexv3 struct{}

func (pdexv3) EstimateTrade(c *gin.Context) {
	sellToken := c.Query("selltoken")
	buyToken := c.Query("buytoken")
	amount := c.Query("amount")
	feeToken := c.Query("feetoken")
	poolID := c.Query("poolid")
	price := c.Query("price")

	_ = sellToken
	_ = buyToken
	_ = amount
	_ = feeToken
	_ = poolID
	_ = price
}
func (pdexv3) ListPairs(c *gin.Context) {
	result, err := database.DBGetPdexPairs()
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
func (pdexv3) GetOrderBook(c *gin.Context) {
	decimal := c.Query("decimal")
	poolID := c.Query("poolid")

	_ = decimal
	_ = poolID
}

func (pdexv3) PendingOrder(c *gin.Context) {
	poolID := c.Query("poolid")
	otakey := c.Query("otakey")

	_ = poolID
	_ = otakey
}

func (pdexv3) ListPools(c *gin.Context) {
	pair := c.Query("pair")
	_ = pair
}

func (pdexv3) PoolsDetail(c *gin.Context) {
	var req struct {
		PoolIDs []string
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
}

func (pdexv3) Share(c *gin.Context) {
	otakey := c.Query("otakey")
	_ = otakey
}

func (pdexv3) WaitingLiquidity(c *gin.Context) {
	otakey := c.Query("otakey")
	_ = otakey
}

func (pdexv3) TradeHistory(c *gin.Context) {
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
			newTxDetail := TxTradeDetail{
				RequestTx:     v.RequestTx,
				RespondTx:     v.RespondTxs,
				Status:        v.Status,
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

func (pdexv3) ContributeHistory(c *gin.Context) {
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
		tx, err := database.DBGetTxByHash([]string{contr.RequestTx})
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

func (pdexv3) WithdrawHistory(c *gin.Context) {
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

func (pdexv3) WithdrawFeeHistory(c *gin.Context) {
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

func (pdexv3) PriceHistory(c *gin.Context) {
	poolid := c.Query("poolid")
	period := c.Query("period")
	datapoint := c.Query("datapoint")

	_ = poolid
	_ = period
	_ = datapoint
}

func (pdexv3) LiquidityHistory(c *gin.Context) {
	poolid := c.Query("poolid")
	period := c.Query("period")
	datapoint := c.Query("datapoint")

	_ = poolid
	_ = period
	_ = datapoint
}
