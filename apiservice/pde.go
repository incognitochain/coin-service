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
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
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

func APIGetWithdrawHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	paymentkey := c.Query("paymentkey")
	// tokenID1 := c.Query("tokenid1")
	// tokenID2 := c.Query("tokenid2")

	result, err := database.DBGetPDEWithdrawRespond(paymentkey, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
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
	// tokenID1 := c.Query("tokenid1")
	// tokenID2 := c.Query("tokenid2")

	result, err := database.DBGetPDEWithdrawFeeRespond(paymentkey, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
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
	// pairID := c.Query("pairid")

	result, err := database.DBGetPDEContributeRespond(paymentkey, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	filterData := make(map[string][]shared.ContributionData)
	for _, data := range result {
		filterData[data.PairID] = append(filterData[data.PairID], data)
	}
	newListData := []shared.ContributionData{}
	for _, dataList := range filterData {
		fiteredList := []shared.ContributionData{}
		waitData := make(map[string]*shared.ContributionData)
		refundData := make(map[string][]shared.ContributionData)
		for _, data := range dataList {
			switch data.Status {
			case common.PDEContributionMatchedChainStatus, common.PDEContributionMatchedNReturnedChainStatus:
				fiteredList = append(fiteredList, data)
			case common.PDEContributionWaitingChainStatus:
				if d, ok := waitData[data.TokenID]; !ok {
					waitData[data.TokenID] = &data
				} else {
					if d.Respondblock < data.Respondblock {
						waitData[data.TokenID] = &data
					}
				}
			case common.PDEContributionRefundChainStatus:
				refundData[data.TokenID] = append(refundData[data.TokenID], data)
			}
		}
		if len(refundData) == 0 {
			if len(fiteredList) != 0 {
				newListData = append(newListData, fiteredList...)
				if len(newListData) == 1 {
					for tokenID, data := range waitData {
						if tokenID != newListData[0].TokenID {
							newListData = append(newListData, *data)
						}
					}
				}
			} else {
				for _, data := range waitData {
					newListData = append(newListData, *data)
				}
			}
		} else {
			for _, data := range refundData {
				newListData = append(newListData, data...)
			}
		}
	}

	sort.SliceStable(newListData, func(i, j int) bool {
		return newListData[i].Respondblock > newListData[j].Respondblock
	})
	respond := APIRespond{
		Result: checkDup(newListData),
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
