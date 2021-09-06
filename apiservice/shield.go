package apiservice

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/wallet"
)

func APIGetUnshieldHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	tokenID := c.Query("tokenid")
	isBase58 := false
	if c.Query("base58") == "true" {
		isBase58 = true
	}
	paymentkey := c.Query("paymentkey")

	pubKeyStr := ""
	if paymentkey == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
		return
	}
	wl, err := wallet.Base58CheckDeserialize(paymentkey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	pubKeyBytes := wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	pubKeyStr = base58.EncodeCheck(pubKeyBytes)
	txDataList, err := database.DBGetTxUnshield(pubKeyStr, tokenID, int64(limit), int64(offset))
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Error: &errStr,
		}
		c.JSON(http.StatusOK, respond)
	}
	result, err := buildTxDetailRespond(txDataList, isBase58)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
	}

	c.JSON(http.StatusOK, respond)
}

func APIGetShieldHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	tokenID := c.Query("tokenid")
	paymentkey := c.Query("paymentkey")

	pubKeyStr := ""
	if paymentkey == "" {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
		return
	}
	wl, err := wallet.Base58CheckDeserialize(paymentkey)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	pubKeyStr = base58.EncodeCheck(wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS())

	txShieldPairlist, err := database.DBGetTxShield(pubKeyStr, tokenID, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	type DataWithLockTime struct {
		TxBridgeDetail
		Locktime int64
	}

	var result []DataWithLockTime
	for _, v := range txShieldPairlist {
		result = append(result, DataWithLockTime{
			TxBridgeDetail: TxBridgeDetail{
				Bridge:          v.Bridge,
				TokenID:         v.TokenID,
				Amount:          v.Amount,
				RespondTx:       v.RespondTx,
				RequestTx:       v.RequestTx,
				IsDecentralized: v.IsDecentralized,
			},
			Locktime: int64(v.RequestTime),
		})
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}

	c.JSON(http.StatusOK, respond)
}
