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
)

func APIGetTradeHistory(c *gin.Context) {
	startTime := time.Now()
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	otakey := c.Query("otakey")
	paymentkey := c.Query("paymentkey")
	key := ""
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
	var result []TxTradeDetail
	for _, v := range txTradePairlist {
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
			Fee:           v.Fee,
			RequestTime:   v.Requesttime,
			BuyToken:      v.BuyTokenID,
			SellToken:     v.SellTokenID,
		}
		newTxDetail.SellAmount, _ = strconv.ParseUint(v.Amount, 10, 64)
		for idx, tk := range v.RespondTokens {
			newTxDetail.ReceiveAmount[tk] = v.RespondAmount[idx]
		}
		result = append(result, newTxDetail)
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
	state, err := database.DBGetPDEState(1)
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

	pubkey, err := extractPubkeyFromKey(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	txList, err := database.DBGetPDETXWithdrawRespond([]string{pubkey}, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	txHashs := []string{}
	for _, v := range txList {
		txHashs = append(txHashs, v.TxHash)
	}
	contrData, err := database.DBGetPDEWithdrawByRespondTx(txHashs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	type DataWithLockTime struct {
		RequestTx   string   `json:"requesttx"`
		RespondTx   []string `json:"respondtx"`
		Status      string   `json:"status"`
		TokenID1    string   `json:"tokenid1"`
		TokenID2    string   `json:"tokenid2"`
		Amount1     uint64   `json:"amount1"`
		Amount2     uint64   `json:"amount2"`
		Contributor string   `json:"contributor"`
		RepondTime  int64    `json:"respondtime"`
		Locktime    int64
	}

	var result []DataWithLockTime
	for _, contr := range contrData {
		statusStr := ""
		switch contr.Status {
		case 0:
			statusStr = "pending"
		case 1:
			statusStr = "accepted"
		case 2:
			statusStr = "rejected"
		}
		amount1, _ := strconv.ParseUint(contr.WithdrawAmount[0], 10, 64)
		amount2, _ := strconv.ParseUint(contr.WithdrawAmount[1], 10, 64)
		data := DataWithLockTime{
			RequestTx:   contr.RequestTx,
			RespondTx:   contr.RespondTxs,
			Status:      statusStr,
			TokenID1:    contr.WithdrawTokens[0],
			TokenID2:    contr.WithdrawTokens[1],
			Amount1:     amount1,
			Amount2:     amount2,
			Contributor: contr.ContributorAddressStr,
			RepondTime:  contr.RequestTime,
			Locktime:    contr.RequestTime,
		}
		result = append(result, data)
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

	pubkey, err := extractPubkeyFromKey(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	txList, err := database.DBGetPDETXWithdrawFeeRespond([]string{pubkey}, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	txHashs := []string{}
	for _, v := range txList {
		txHashs = append(txHashs, v.TxHash)
	}
	contrData, err := database.DBGetPDEWithdrawFeeByRespondTx(txHashs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	type DataWithLockTime struct {
		RequestTx   string `json:"requesttx"`
		RespondTx   string `json:"respondtx"`
		Status      string `json:"status"`
		TokenID1    string `json:"tokenid1"`
		TokenID2    string `json:"tokenid2"`
		Amount      uint64 `json:"amount"`
		Contributor string `json:"contributor"`
		RepondTime  int64  `json:"respondtime"`
		Locktime    int64
	}

	var result []DataWithLockTime
	for _, contr := range contrData {
		statusStr := ""
		switch contr.Status {
		case 0:
			statusStr = "pending"
		case 1:
			statusStr = "accepted"
		case 2:
			statusStr = "rejected"
		}
		amount, _ := strconv.ParseUint(contr.WithdrawAmount[0], 10, 64)
		tx, _ := database.DBGetTxByHash([]string{contr.RequestTx})
		md := metadata.PDEFeeWithdrawalRequest{}
		err := json.UnmarshalFromString(tx[0].Metadata, &md)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		data := DataWithLockTime{
			RequestTx:   contr.RequestTx,
			RespondTx:   contr.RespondTxs[0],
			Status:      statusStr,
			TokenID1:    md.WithdrawalToken1IDStr,
			TokenID2:    md.WithdrawalToken2IDStr,
			Amount:      amount,
			Contributor: md.WithdrawerAddressStr,
			RepondTime:  contr.RequestTime,
			Locktime:    contr.RequestTime,
		}
		result = append(result, data)
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

	// wl, err := wallet.Base58CheckDeserialize(paymentkey)
	pubkey, err := extractPubkeyFromKey(paymentkey, false)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	contrData, err := database.DBGetPDEContributeRespond(pubkey, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	contrDataNoDup := checkDup(contrData)
	type DataWithLockTime struct {
		ContributionDataV1
		Locktime int64
	}
	var result []DataWithLockTime
	for _, contr := range contrDataNoDup {
		for idx, v := range contr.RequestTxs {
			newData := DataWithLockTime{}
			tx, err := database.DBGetTxByHash([]string{v})
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			newData.Locktime = tx[0].Locktime
			newData.Amount, _ = strconv.ParseUint(contr.ContributeAmount[idx], 10, 64)
			newData.TokenID = contr.ContributeTokens[idx]
			newData.ContributorAddressStr = contr.Contributor
			newData.PairID = contr.PairID
			newData.RequestTx = v
			statusText := "waiting"
			if idx != 0 && (contr.ContributeTokens[0] != contr.ContributeTokens[1]) && len(contr.ReturnTokens) == 0 {
				if len(contr.RespondTxs) == 0 {
					statusText = "matched"
				}
				// ctk := contr.ContributeTokens[idx]
				// for _, v := range contr.ReturnTokens {
				// 	if v == ctk {
				// 		break
				// 	}
				// }
			}
			if idx != 0 && (contr.ContributeTokens[0] == contr.ContributeTokens[1]) {
				continue
			}

			newData.Status = statusText
			result = append(result, newData)
		}
		for idx, v := range contr.RespondTxs {
			newData := DataWithLockTime{}
			tx, err := database.DBGetTxByHash([]string{v})
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			newData.Locktime = tx[0].Locktime
			newData.Status = "matchedNReturned"
			if contr.ContributeTokens[0] == contr.ContributeTokens[1] {
				newData.Status = "refund"
			}

			tk := contr.ReturnTokens[idx]
			newData.TokenID = tk
			newData.ReturnAmount, _ = strconv.ParseUint(contr.ReturnAmount[idx], 10, 64)
			if newData.Status == "matchedNReturned" {
				a, _ := strconv.ParseUint(contr.ContributeAmount[idx], 10, 64)
				newData.Amount = a - newData.ReturnAmount
			}
			for idxtk, v := range contr.ContributeTokens {
				if v == tk {
					newData.RequestTx = contr.RequestTxs[idxtk]
					break
				}
			}
			newData.RespondTx = v
			newData.PairID = contr.PairID
			newData.ContributorAddressStr = contr.Contributor
			result = append(result, newData)
		}

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
