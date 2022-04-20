package apiservice

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/blockchain/bridgeagg"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
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

	// pubKeyStr := ""
	// if paymentkey == "" {
	// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
	// 	return
	// }
	// wl, err := wallet.Base58CheckDeserialize(paymentkey)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 	return
	// }
	// pubKeyBytes := wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS()
	// pubKeyStr = base58.EncodeCheck(pubKeyBytes)
	pubKeyStr, err := extractPubkeyFromKey(paymentkey, false)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
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

	// pubKeyStr := ""
	// if paymentkey == "" {
	// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("PaymentKey cant be empty")))
	// 	return
	// }
	// wl, err := wallet.Base58CheckDeserialize(paymentkey)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 	return
	// }
	// pubKeyStr = base58.EncodeCheck(wl.KeySet.PaymentAddress.GetPublicSpend().ToBytesS())

	pubKeyStr, err := extractPubkeyFromKey(paymentkey, false)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

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
		amount, _ := strconv.ParseUint(v.Amount, 10, 64)
		result = append(result, DataWithLockTime{
			TxBridgeDetail: TxBridgeDetail{
				Bridge:          v.Bridge,
				TokenID:         v.TokenID,
				Amount:          amount,
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

func APIGetTxShield(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	fromtime, _ := strconv.Atoi(c.Query("fromtime"))
	// fromtime, err := strconv.ParseUint(fromtimeStr, 10, 64)
	// if err != nil {
	// 	errStr := err.Error()
	// 	respond := APIRespond{
	// 		Result: nil,
	// 		Error:  &errStr,
	// 	}

	// 	c.JSON(http.StatusOK, respond)
	// 	return
	// }
	fmt.Println("APIGetTxShield", offset, fromtime)
	list, err := database.DBGetShieldWithRespond(uint64(fromtime), int64(offset))
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}

		c.JSON(http.StatusOK, respond)
		return
	}
	respond := APIRespond{
		Result: list,
		Error:  nil,
	}

	c.JSON(http.StatusOK, respond)
}

func APIGetBridgeAggState(c *gin.Context) {
	state, err := database.DBGetBridgeState(1)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	bridgeState := jsonresult.BridgeAggState{}
	err = json.UnmarshalFromString(state, &bridgeState)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: bridgeState,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func APIGetSupportedVault(c *gin.Context) {
	unifiedTokenID := c.Query("punified")
	burntAmountStr := c.Query("burntamount")
	expectedAmountStr := c.Query("expectedamount")
	if bridgeState == nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("Bridge state is nil")))
		return
	}
	var err error
	burntAmount := uint64(0)
	expectedAmount := uint64(0)
	if burntAmountStr != "" {
		burntAmount, err = strconv.ParseUint(burntAmountStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		expectedAmount, err = strconv.ParseUint(expectedAmountStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	if burntAmount <= 0 && expectedAmount <= 0 {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("BurntAmount and ExpectedAmount cant be both <= 0")))
		return
	}
	var result struct {
		Burn     map[uint]jsonresult.BridgeAggEstimateFee
		Expected map[uint]jsonresult.BridgeAggEstimateFee
	}
	resultBurn := make(map[uint]jsonresult.BridgeAggEstimateFee)
	resultExpected := make(map[uint]jsonresult.BridgeAggEstimateFee)

	unifiedTokenIDHash := common.Hash{}.NewHashFromStr2(unifiedTokenID)
	if networkIDs, ok := bridgeState.UnifiedTokenInfos()[unifiedTokenIDHash]; ok {
		for networkID, vault := range networkIDs {
			x := vault.Reserve()
			y := vault.CurrentRewardReserve()
			if burntAmount != 0 {
				exAmount, err := bridgeagg.EstimateActualAmountByBurntAmount(x, y, burntAmount, vault.IsPaused())
				if err != nil {
					c.JSON(http.StatusOK, buildGinErrorRespond(err))
					return
				}
				resultBurn[networkID] = jsonresult.BridgeAggEstimateFee{
					ExpectedAmount: exAmount,
					Fee:            burntAmount - expectedAmount,
					BurntAmount:    burntAmount,
				}
			} else {
				fee, err := bridgeagg.CalculateDeltaY(x, y, expectedAmount, bridgeagg.SubOperator, vault.IsPaused())
				if err != nil {
					c.JSON(http.StatusOK, buildGinErrorRespond(err))
					return
				}
				if expectedAmount > x {
					c.JSON(http.StatusOK, buildGinErrorRespond(fmt.Errorf("Unshield amount %v > vault amount %v", expectedAmount, x)))
					return
				}
				burntAmount := big.NewInt(0).Add(big.NewInt(0).SetUint64(fee), big.NewInt(0).SetUint64(expectedAmount))
				if !burntAmount.IsUint64() {
					c.JSON(http.StatusOK, buildGinErrorRespond(fmt.Errorf("Value is not unit64")))
					return
				}
				resultBurn[networkID] = jsonresult.BridgeAggEstimateFee{
					ExpectedAmount: expectedAmount,
					Fee:            fee,
					BurntAmount:    burntAmount.Uint64(),
				}
			}
		}
	} else {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("UnifiedTokenID not found")))
		return
	}
	if len(resultBurn) == 0 && len(resultExpected) == 0 {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("No supported vault")))
		return
	}
	result.Burn = resultBurn
	result.Expected = resultExpected
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
