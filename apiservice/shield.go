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
	pubKeyBytes := []byte{}
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
	// otakey := c.Query("otakey")
	tokenID := c.Query("tokenid")
	paymentkey := c.Query("paymentkey")

	pubKeyStr := ""
	pubKeyBytes := []byte{}
	// if otakey != "" {
	// 	wl, err := wallet.Base58CheckDeserialize(otakey)
	// 	if err != nil {
	// 		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 		return
	// 	}
	// 	pubKeyBytes = wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	// } else {
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
	// }
	pubKeyStr = base58.EncodeCheck(pubKeyBytes)

	list, err := database.DBGetTxShieldRespond(pubKeyStr, tokenID, int64(limit), int64(offset))
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

	txShieldPairlist, err := database.DBGetTxShield(respList)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var result []*TxBridgeDetail
	for _, v := range txShieldPairlist {
		result = append(result, &TxBridgeDetail{
			Bridge:          v.Bridge,
			TokenID:         v.TokenID,
			Amount:          v.Amount,
			RespondTx:       v.RespondTx,
			RequestTx:       v.RequestTx,
			ShieldType:      v.ShieldType,
			IsDecentralized: v.IsDecentralized,
		})
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}

	c.JSON(http.StatusOK, respond)
}
