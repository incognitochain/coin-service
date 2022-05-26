package apiservice

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/pdexv3/feeestimator"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/metadata"
	pdexv3Meta "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

type pdexv3 struct{}

func (pdexv3) ListPairs(c *gin.Context) {
	var result []PdexV3PairData
	list, err := database.DBGetPdexPairs()
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	for _, v := range list {

		tk1Amount, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
		data := PdexV3PairData{
			PairID:       v.PairID,
			TokenID1:     v.TokenID1,
			TokenID2:     v.TokenID2,
			Token1Amount: tk1Amount,
			Token2Amount: tk2Amount,
			PoolCount:    v.PoolCount,
		}
		result = append(result, data)
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) ListPools(c *gin.Context) {
	pair := c.Query("pair")
	verify := c.Query("verify")
	var result []PdexV3PoolDetail
	if pair == "all" {
		if verify == "true" {
			result = getPoolList(true)
		} else {
			result = getPoolList(false)
		}
	} else {
		if verify == "true" {
			getPoolListByPairID(pair, true)
		} else {
			getPoolListByPairID(pair, false)
		}
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) TradeStatus(c *gin.Context) {
	requestTx := c.Query("requesttx")
	tradeInfo, tradeStatus, err := database.DBGetTradeInfoAndStatus(requestTx)
	if err != nil && tradeInfo == nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(tradeInfo, tradeStatus)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
	minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
	result := TradeDataRespond{
		RequestTx:           tradeInfo.RequestTx,
		RespondTxs:          tradeInfo.RespondTxs,
		RespondTokens:       tradeInfo.RespondTokens,
		RespondAmounts:      tradeInfo.RespondAmount,
		WithdrawTxs:         withdrawTxs,
		PoolID:              tradeInfo.PoolID,
		PairID:              tradeInfo.PairID,
		SellTokenID:         tradeInfo.SellTokenID,
		BuyTokenID:          tradeInfo.BuyTokenID,
		Amount:              amount,
		MinAccept:           minAccept,
		Matched:             matchedAmount,
		Status:              status,
		StatusCode:          statusCode,
		Requestime:          tradeInfo.Requesttime,
		NFTID:               tradeInfo.NFTID,
		Fee:                 tradeInfo.Fee,
		FeeToken:            tradeInfo.FeeToken,
		Receiver:            tradeInfo.Receiver,
		IsCompleted:         isCompleted,
		SellTokenBalance:    sellTokenBl,
		BuyTokenBalance:     buyTokenBl,
		SellTokenWithdrawed: sellTokenWD,
		BuyTokenWithdrawed:  buyTokenWD,
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PoolShare(c *gin.Context) {
	nftID := c.Query("nftid")
	otakey := c.Query("otakey")
	if nftID == "" && otakey == "" {
		errStr := "nftID/otakey can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	var list []shared.PoolShareData
	var err error
	isNextOTA := false
	rawOTA64 := make(map[string]string)
	if otakey != "" {
		accessOTAList, raw64, err := retrieveAccessOTAList(otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		if len(accessOTAList) == 0 {
			respond := APIRespond{
				Result: nil,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
			return
		}
		// list, err = database.DBGetShareByCurrentAccessID(accessOTAList)
		// if err != nil {
		// 	c.JSON(http.StatusOK, buildGinErrorRespond(err))
		// 	return
		// }
		list, err = database.DBGetShare(accessOTAList)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		// list = append(list, list...)
		rawOTA64 = raw64
		isNextOTA = true
	} else {
		list, err = database.DBGetShare([]string{nftID})
		if err != nil {
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
		}
	}

	result, err := producePoolShareRespond(list, isNextOTA, rawOTA64)
	if err != nil {
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) TradeHistory(c *gin.Context) {
	startTime := time.Now()
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	poolid := c.Query("poolid")
	nftID := c.Query("nftid")
	isOrder := c.Query("isorder")
	getOrder := false
	otakey := c.Query("otakey")
	if nftID == "" && otakey == "" {
		errStr := "nftID/otakey can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	if otakey != "" && isOrder == "true" {
		getOrder = true
	}
	var result []TradeDataRespond
	if otakey != "" {
		if getOrder {
			if poolid == "" {
				errStr := "poolid can't be empty"
				respond := APIRespond{
					Result: nil,
					Error:  &errStr,
				}
				c.JSON(http.StatusOK, respond)
				return
			}
			accessOTAList, rawOTA64, err := retrieveAccessOTAList(otakey)
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			//limit order
			tradeList, err := database.DBGetTxTradeFromPoolAndAccessID(poolid, accessOTAList, int64(limit), int64(offset))
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			txRequest := []string{}
			for _, tx := range tradeList {
				txRequest = append(txRequest, tx.RequestTx)
			}
			tradeStatusList, err := database.DBGetTradeStatus(txRequest)
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			result, err = produceTradeDataRespond(tradeList, tradeStatusList, rawOTA64)
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
		} else {
			pubkey, err := extractPubkeyFromKey(otakey, true)
			if err != nil {
				errStr := err.Error()
				respond := APIRespond{
					Result: nil,
					Error:  &errStr,
				}
				c.JSON(http.StatusOK, respond)
				return
			}
			fmt.Println("pubkey, metadata.Pdexv3TradeRequestMeta", pubkey, metadata.Pdexv3TradeRequestMeta)
			txList, err := database.DBGetTxByMetaAndOTA(pubkey, metadata.Pdexv3TradeRequestMeta, int64(limit), int64(offset))
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			txRequest := []string{}
			for _, tx := range txList {
				txRequest = append(txRequest, tx.TxHash)
			}
			list, err := database.DBGetTxTradeFromTxRequest(txRequest)
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			for _, tradeInfo := range list {
				matchedAmount := uint64(0)
				status := ""
				isCompleted := false
				switch tradeInfo.Status {
				case 0:
					status = "pending"
				case 1:
					status = "accepted"
					matchedAmount, _ = strconv.ParseUint(tradeInfo.Amount, 10, 64)
					isCompleted = true
				case 2:
					status = "rejected"
					isCompleted = true
				}
				amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
				minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
				uniqIdx := getUniqueIdx(tradeInfo.RespondTxs)
				trade := TradeDataRespond{
					RequestTx:   tradeInfo.RequestTx,
					WithdrawTxs: nil,
					PoolID:      tradeInfo.PoolID,
					PairID:      tradeInfo.PairID,
					SellTokenID: tradeInfo.SellTokenID,
					BuyTokenID:  tradeInfo.BuyTokenID,
					Amount:      amount,
					MinAccept:   minAccept,
					Matched:     matchedAmount,
					Status:      status,
					StatusCode:  tradeInfo.Status,
					Requestime:  tradeInfo.Requesttime,
					NFTID:       tradeInfo.NFTID,
					Fee:         tradeInfo.Fee,
					FeeToken:    tradeInfo.FeeToken,
					Receiver:    tradeInfo.Receiver,
					IsCompleted: isCompleted,
					TradingPath: tradeInfo.TradingPath,
				}
				for _, v := range uniqIdx {
					trade.RespondTxs = append(trade.RespondTxs, tradeInfo.RespondTxs[v])
					trade.RespondTokens = append(trade.RespondTokens, tradeInfo.RespondTokens[v])
					trade.RespondAmounts = append(trade.RespondAmounts, tradeInfo.RespondAmount[v])
				}
				result = append(result, trade)
			}
		}
	} else {
		//limit order
		tradeList, err := database.DBGetTxTradeFromPoolAndNFT(poolid, nftID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		txRequest := []string{}
		for _, tx := range tradeList {
			txRequest = append(txRequest, tx.RequestTx)
		}
		tradeStatusList, err := database.DBGetTradeStatus(txRequest)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		result, err = produceTradeDataRespond(tradeList, tradeStatusList, nil)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}

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
	nftID := c.Query("nftid")
	otakey := c.Query("otakey")
	if nftID == "" && otakey == "" {
		errStr := "nftID/otakey can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

	var err error
	var list []shared.ContributionData
	isNextOTA := false
	rawOTA64 := map[string]string{}
	if otakey == "" {
		list, err = database.DBGetPDEV3ContributeRespond([]string{nftID}, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		isNextOTA = true
		accessIDList, raw64, err := retrieveAccessOTAList(otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		list, err = database.DBGetPDEV3ContributeRespondByAccessID(accessIDList, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		respondTxs := []string{}
		for _, v := range list {
			respondTxs = append(respondTxs, v.RespondTxs...)
		}
		rList, err := getRefundContribution(otakey, respondTxs)
		if err != nil {
			log.Println("getRefundContribution", err)
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		wList, err := getNextOTAWaitingContribution(otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		list = append(list, rList...)
		list = append(list, wList...)
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].RequestTime > list[j].RequestTime
		})
		rawOTA64 = raw64
	}

	result, err := produceContributeData(list, isNextOTA, rawOTA64)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) WaitingLiquidity(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	// poolid := c.Query("poolid")
	nftid := c.Query("nftid")

	result, err := database.DBGetPDEV3ContributeWaiting(nftid, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) WithdrawHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	poolID := c.Query("poolid")
	nftID := c.Query("nftid")
	otakey := c.Query("otakey")
	if nftID == "" && otakey == "" {
		errStr := "otakey/nftid can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	accessIDList := []string{}
	if otakey != "" {
		var err error
		accessIDList, _, err = retrieveAccessOTAList(otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		accessIDList = append(accessIDList, nftID)
	}
	var err error
	var list []shared.WithdrawContributionData

	if poolID != "" {
		list, err = database.DBGetPDEV3WithdrawRespond(accessIDList, poolID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		list, err = database.DBGetPDEV3WithdrawRespond(accessIDList, "", int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	result, err := produceWithdrawContributeData(list)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) WithdrawFeeHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	poolID := c.Query("poolid")
	nftID := c.Query("nftid")
	otakey := c.Query("otakey")
	if nftID == "" && otakey == "" {
		errStr := "otakey/nftid can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	accessIDList := []string{}
	if otakey != "" {
		var err error
		accessIDList, _, err = retrieveAccessOTAList(otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		accessIDList = append(accessIDList, nftID)
	}
	if len(accessIDList) == 0 {
		errStr := "no accessID found"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	var result []PdexV3WithdrawFeeRespond
	var err error
	var list []shared.WithdrawContributionFeeData

	if poolID != "" {
		list, err = database.DBGetPDEV3WithdrawFeeRespond(accessIDList, poolID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	} else {
		list, err = database.DBGetPDEV3WithdrawFeeRespond(accessIDList, "", int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}

	for _, v := range list {
		tokens := make(map[string]uint64)
		for idx, tk := range v.WithdrawTokens {
			tokens[tk], _ = strconv.ParseUint(v.WithdrawAmount[idx], 10, 64)
		}
		result = append(result, PdexV3WithdrawFeeRespond{
			PoolID:         v.PoodID,
			RequestTx:      v.RequestTx,
			RespondTxs:     v.RespondTxs,
			WithdrawTokens: tokens,
			Status:         v.Status,
			Requestime:     v.RequestTime,
		})
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) StakingPool(c *gin.Context) {
	list, err := database.DBGetStakePools()
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	var result []PdexV3StakingPoolInfo
	for _, v := range list {

		apy, err := database.DBGetPDEPoolPairRewardAPY(v.TokenID)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		data := PdexV3StakingPoolInfo{
			Amount:  v.Amount,
			TokenID: v.TokenID,
			APY:     int(apy.APY2),
		}
		result = append(result, data)
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) StakeInfo(c *gin.Context) {
	nftID := c.Query("nftid")
	accessID := c.Query("accessid")
	if nftID == "" && accessID == "" {
		errStr := "accessID/nftID can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	if accessID == "" {
		accessID = nftID
	}
	list, err := database.DBGetStakingInfo(accessID)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var result []shared.PoolStakerData
	for _, v := range list {
		if v.Amount != 0 || len(v.Reward) != 0 {
			result = append(result, v)
		}
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) StakeHistory(c *gin.Context) {

	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	tokenid := c.Query("tokenid")
	nftID := c.Query("nftid")
	accessID := c.Query("accessid")
	if nftID == "" && accessID == "" {
		errStr := "accessID/nftID can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	if accessID == "" {
		accessID = nftID
	}
	list, err := database.DBGetStakingPoolHistory(accessID, tokenid, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var result []PdexV3StakingPoolHistoryData
	for _, v := range list {
		data := PdexV3StakingPoolHistoryData{
			IsStaking:   v.IsStaking,
			RequestTx:   v.RequestTx,
			RespondTx:   v.RespondTx,
			Status:      v.Status,
			TokenID:     v.TokenID,
			NFTID:       v.NFTID,
			Amount:      v.Amount,
			Requesttime: v.Requesttime,
		}
		result = append(result, data)
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) StakeRewardHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	tokenid := c.Query("tokenid")
	nftID := c.Query("nftid")
	accessID := c.Query("accessid")
	if nftID == "" && accessID == "" {
		errStr := "accessID/nftID can't be empty"
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	if accessID == "" {
		accessID = nftID
	}
	list, err := database.DBGetStakePoolRewardHistory(accessID, tokenid, int64(limit), int64(offset))
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var result []PdexV3StakePoolRewardHistoryData
	for _, v := range list {
		rewards := make(map[string]uint64)
		for idx, tk := range v.RewardTokens {
			rewards[tk] = v.Amount[idx]
		}
		data := PdexV3StakePoolRewardHistoryData{
			RespondTxs:   v.RespondTxs,
			RequestTx:    v.RequestTx,
			Status:       v.Status,
			TokenID:      v.TokenID,
			NFTID:        v.NFTID,
			RewardTokens: rewards,
			Requesttime:  v.Requesttime,
		}
		result = append(result, data)
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PairsDetail(c *gin.Context) {
	var req struct {
		PairIDs []string
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	list, err := database.DBGetPairsByID(req.PairIDs)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var result []PdexV3PairData
	for _, v := range list {
		tk1Amount, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
		data := PdexV3PairData{
			PairID:       v.PairID,
			TokenID1:     v.TokenID1,
			TokenID2:     v.TokenID2,
			Token1Amount: tk1Amount,
			Token2Amount: tk2Amount,
			PoolCount:    v.PoolCount,
		}
		result = append(result, data)
	}
	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PoolsDetail(c *gin.Context) {
	var req struct {
		PoolIDs []string
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	result := getCustomPoolList(req.PoolIDs)
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) GetOrderBook(c *gin.Context) {
	decimal := c.Query("decimal")
	poolID := c.Query("poolid")

	decimalFloat, err := strconv.ParseFloat(decimal, 64)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var priorityTokens []string
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	tks := strings.Split(poolID, "-")
	pairID := tks[0] + "-" + tks[1]
	list, err := database.DBGetPendingOrderByPairID(pairID)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	willSwap := willSwapTokenPlace(tks[0], tks[1], priorityTokens)
	_ = willSwap
	var result PdexV3OrderBookRespond
	var sellSide []shared.TradeOrderData
	var buySide []shared.TradeOrderData
	for _, v := range list {
		if v.SellTokenID == tks[0] {
			sellSide = append(sellSide, v)
		} else {
			buySide = append(buySide, v)
		}
	}
	sellVolume := make(map[string]PdexV3OrderBookVolume)
	buyVolume := make(map[string]PdexV3OrderBookVolume)

	for _, v := range sellSide {
		amount, _ := strconv.ParseUint(v.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(v.MinAccept, 10, 64)
		price := float64(minAccept) / float64(amount)
		group := math.Floor(float64(price)*(1/decimalFloat)) * decimalFloat
		groupStr := fmt.Sprintf("%g", group)
		if d, ok := sellVolume[groupStr]; !ok {
			sellVolume[groupStr] = PdexV3OrderBookVolume{
				Price:   group,
				average: price,
				Volume:  amount,
			}
		} else {
			d.Volume += amount
			a := (d.average + price) / 2
			d.average = a
			sellVolume[groupStr] = d
		}
	}

	for _, v := range buySide {
		amount, _ := strconv.ParseUint(v.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(v.MinAccept, 10, 64)
		price := float64(amount) / float64(minAccept)
		group := math.Floor(float64(price)*(1/decimalFloat)) * decimalFloat
		groupStr := fmt.Sprintf("%g", group)
		if d, ok := buyVolume[groupStr]; !ok {
			buyVolume[groupStr] = PdexV3OrderBookVolume{
				Price:   group,
				average: price,
				Volume:  amount,
			}
		} else {
			d.Volume += amount
			a := (d.average + price) / 2
			d.average = a
			buyVolume[groupStr] = d
		}
	}

	for _, v := range sellVolume {
		data := PdexV3OrderBookVolume{
			Price:  v.Price,
			Volume: v.Volume,
		}
		result.Sell = append(result.Sell, data)
	}
	for _, v := range buyVolume {
		data := PdexV3OrderBookVolume{
			Price:  v.Price,
			Volume: v.Volume,
		}
		result.Buy = append(result.Buy, data)
	}
	sort.SliceStable(result.Sell, func(i, j int) bool {
		return result.Sell[i].Price > result.Sell[j].Price
	})
	sort.SliceStable(result.Buy, func(i, j int) bool {
		return result.Buy[i].Price > result.Buy[j].Price
	})
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) GetLatestTradeOrders(c *gin.Context) {
	isswap := c.Query("isswap")
	getSwap := false
	if isswap == "true" {
		getSwap = true
	}
	result, err := database.DBGetLatestTradeTx(getSwap)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) EstimateTrade(c *gin.Context) {
	var req struct {
		SellToken  string `form:"selltoken" json:"selltoken" binding:"required"`
		BuyToken   string `form:"buytoken" json:"buytoken" binding:"required"`
		SellAmount uint64 `form:"sellamount" json:"sellamount"`
		BuyAmount  uint64 `form:"buyamount" json:"buyamount"`
		Pdecimal   bool   `form:"pdecimal" json:"pdecimal"`
		IsMax      bool   `form:"ismax" json:"ismax"`
	}
	err := c.ShouldBindQuery(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	sellToken := req.SellToken
	buyToken := req.BuyToken
	// feeInPRV := req.FeeInPRV
	sellAmount := req.SellAmount
	buyAmount := req.BuyAmount

	if sellAmount <= 0 && buyAmount <= 0 {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("sellAmount and buyAmount must be >=0")))
		return
	}
	if sellAmount > 0 && buyAmount > 0 {
		c.JSON(http.StatusOK, buildGinErrorRespond(errors.New("only accept sellAmount or buyAmount value individually")))
		return
	}

	var result PdexV3EstimateTradeRespondBig
	var feePRV PdexV3EstimateTradeRespond
	var feeToken PdexV3EstimateTradeRespond

	state, err := database.DBGetPDEState(2)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	data := `{"Result":` + state + `}`

	var responseBodyData shared.Pdexv3GetStateRPCResult

	err = json.UnmarshalFromString(data, &responseBodyData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	pools, poolPairStates, err := pathfinder.GetPdexv3PoolDataFromRawRPCResult(responseBodyData.Result.Poolpairs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	pdexState, err := feeestimator.GetPdexv3PoolDataFromRawRPCResult(responseBodyData.Result.Params, responseBodyData.Result.Poolpairs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	// dcrate := float64(1)
	dcrate, tk1Decimal, _, err := getPdecimalRate(buyToken, sellToken)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	// _ = tk2Decimal
	if !req.Pdecimal {
		dcrate = 1
	}

	var defaultPools map[string]struct{}
	if err := cacheGet(defaultPoolsKey, &defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool(true)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(defaultPoolsKey, defaultPools)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	var newPools []*shared.Pdexv3PoolPairWithId
	for _, pool := range pools {
		if _, ok := defaultPools[pool.PoolID]; ok {
			newPools = append(newPools, pool)
		} else {
			delete(poolPairStates, pool.PoolID)
			delete(pdexState.PoolPairs, pool.PoolID)
		}
	}

	// var chosenPath []*shared.Pdexv3PoolPairWithId
	// var foundSellAmount uint64
	// var receive uint64

	if sellAmount > 0 {
		//feePRV
		chosenPath, receive := pathfinder.FindGoodTradePath(
			pdexv3Meta.MaxTradePathLength,
			newPools,
			poolPairStates,
			sellToken,
			buyToken,
			sellAmount)
		feePRV.SellAmount = float64(sellAmount) / dcrate
		feePRV.MaxGet = float64(receive) * dcrate

		feeToken.SellAmount = float64(sellAmount) / dcrate
		feeToken.MaxGet = float64(receive) * dcrate

		if chosenPath != nil {
			feePRV.Route = make([]string, 0)
			for _, v := range chosenPath {
				feePRV.Route = append(feePRV.Route, v.PoolID)
			}
			tradingFeePRV, err := feeestimator.EstimateTradingFee(uint64(sellAmount), sellToken, feePRV.Route, *pdexState, true)
			if err != nil {
				log.Print("can not estimate fee: ", err)
				// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
				// return
			}
			feePRV.Fee = tradingFeePRV + (tradingFeePRV / 100)
			if req.IsMax && sellToken == common.PRVCoinID.String() && feePRV.Fee > 0 {
				newSellAmount := sellAmount - tradingFeePRV - 100 - (tradingFeePRV / 100)
				chosenPath, receive := pathfinder.FindGoodTradePath(
					pdexv3Meta.MaxTradePathLength,
					newPools,
					poolPairStates,
					sellToken,
					buyToken,
					newSellAmount)
				feePRV.SellAmount = float64(newSellAmount) / dcrate
				feePRV.MaxGet = float64(receive) * dcrate
				feePRV.Route = make([]string, 0)
				for _, v := range chosenPath {
					feePRV.Route = append(feePRV.Route, v.PoolID)
				}
				tradingFeePRV, err := feeestimator.EstimateTradingFee(uint64(newSellAmount), sellToken, feePRV.Route, *pdexState, true)
				if err != nil {
					log.Print("can not estimate fee: ", err)
					// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
					// return
				}
				feePRV.Fee = tradingFeePRV + (tradingFeePRV / 100)
			}
			feePRV.TokenRoute = getTokenRoute(sellToken, feePRV.Route)
		}

		//feeToken
		if sellToken == common.PRVCoinID.String() {
			feeToken = feePRV
		} else {
			if chosenPath != nil {
				feeToken.Route = make([]string, 0)
				for _, v := range chosenPath {
					feeToken.Route = append(feeToken.Route, v.PoolID)
				}
				tradingFeeToken, err := feeestimator.EstimateTradingFee(uint64(sellAmount), sellToken, feeToken.Route, *pdexState, false)
				if err != nil {
					log.Print("can not estimate fee: ", err)
					// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
					// return
				}
				feeToken.Fee = tradingFeeToken
				if req.IsMax && feeToken.Fee > 0 {
					newSellAmount := sellAmount - tradingFeeToken
					chosenPath, receive := pathfinder.FindGoodTradePath(
						pdexv3Meta.MaxTradePathLength,
						newPools,
						poolPairStates,
						sellToken,
						buyToken,
						newSellAmount)
					feeToken.SellAmount = float64(newSellAmount) / dcrate
					feeToken.MaxGet = float64(receive) * dcrate
					feeToken.Route = make([]string, 0)
					for _, v := range chosenPath {
						feeToken.Route = append(feeToken.Route, v.PoolID)
					}
					tradingFeeToken, err := feeestimator.EstimateTradingFee(uint64(newSellAmount), sellToken, feeToken.Route, *pdexState, false)
					if err != nil {
						log.Print("can not estimate fee: ", err)
						// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
						// return
					}
					feeToken.Fee = tradingFeeToken
				}
				feeToken.TokenRoute = getTokenRoute(sellToken, feeToken.Route)
			}
		}
	} else {
		chosenPath, foundSellAmount := pathfinder.FindSellAmount(
			pdexv3Meta.MaxTradePathLength,
			newPools,
			poolPairStates,
			sellToken,
			buyToken,
			buyAmount)
		sellAmount = foundSellAmount
		receive := buyAmount

		feePRV.SellAmount = float64(foundSellAmount) / dcrate
		feePRV.MaxGet = float64(receive) * dcrate
		feeToken.SellAmount = float64(foundSellAmount) / dcrate
		feeToken.MaxGet = float64(receive) * dcrate

		if chosenPath != nil {
			feePRV.Route = make([]string, 0)
			for _, v := range chosenPath {
				feePRV.Route = append(feePRV.Route, v.PoolID)
			}
			feeToken.Route = feePRV.Route
			tradingFeePRV, err := feeestimator.EstimateTradingFee(uint64(sellAmount), sellToken, feePRV.Route, *pdexState, true)
			if err != nil {
				log.Print("can not estimate fee: ", err)
				// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
				// return
			}
			feePRV.Fee = tradingFeePRV + (tradingFeePRV / 100)
			feePRV.TokenRoute = getTokenRoute(sellToken, feePRV.Route)
			tradingFeeToken, err := feeestimator.EstimateTradingFee(uint64(sellAmount), sellToken, feePRV.Route, *pdexState, false)
			if err != nil {
				log.Print("can not estimate fee: ", err)
				// c.JSON(http.StatusUnprocessableEntity, buildGinErrorRespond(errors.New("can not estimate fee: "+err.Error())))
				// return
			}
			feeToken.Fee = tradingFeeToken
			feeToken.TokenRoute = getTokenRoute(sellToken, feeToken.Route)
		}
	}
	if feePRV.Fee != 0 {
		rt := getRateMinimum(buyToken, sellToken, uint64(math.Pow10(tk1Decimal)), newPools, poolPairStates)
		if rt == 0 {
			rt = getRateMinimum(buyToken, sellToken, 1, pools, poolPairStates)
			if rt == 0 {
				rt = feePRV.MaxGet / feePRV.SellAmount
			}
		}
		rt1 := feePRV.SellAmount / feePRV.MaxGet
		ia := (1 - (rt / rt1)) * 100
		if ia >= 20 {
			feePRV.IsSignificant = true
		}
		feePRV.ImpactAmount = ia
	}
	if feeToken.Fee != 0 {
		rt := getRateMinimum(buyToken, sellToken, uint64(math.Pow10(tk1Decimal)), newPools, poolPairStates)
		if rt == 0 {
			rt = getRateMinimum(buyToken, sellToken, 1, pools, poolPairStates)
			if rt == 0 {
				rt = feeToken.MaxGet / feeToken.SellAmount
			}
		}
		rt1 := feeToken.SellAmount / feeToken.MaxGet
		ia := (1 - (rt / rt1)) * 100
		if ia >= 20 {
			feeToken.IsSignificant = true
		}
		feeToken.ImpactAmount = ia
	}
	result.FeePRV = feePRV
	result.FeeToken = feeToken
	var errStr *string
	if feePRV.Fee == 0 && feeToken.Fee == 0 {
		e := "no trade route found"
		errStr = &e
	}
	respond := APIRespond{
		Result: result,
		Error:  errStr,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PriceHistory(c *gin.Context) {
	poolid := c.Query("poolid")
	period := c.Query("period")
	intervals := c.Query("intervals")

	analyticsData, err := analyticsquery.APIGetPDexV3PairRateHistories(poolid, period, intervals)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	var priorityTokens []string
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}
	tokenIDs := strings.Split(poolid, "-")
	token1ID := tokenIDs[0]
	token2ID := tokenIDs[1]
	var result []PdexV3PriceHistoryRespond

	willSwap := willSwapTokenPlace(token1ID, token2ID, priorityTokens)
	for _, v := range analyticsData.Result {
		tm, _ := time.Parse(time.RFC3339, v.Timestamp)
		if willSwap {
			var pdexV3PriceHistoryRespond = PdexV3PriceHistoryRespond{
				Timestamp: tm.Unix(),
				High:      1 / v.Low,
				Low:       1 / v.High,
				Open:      1 / v.Open,
				Close:     1 / v.Close,
			}
			result = append(result, pdexV3PriceHistoryRespond)
		} else {
			var pdexV3PriceHistoryRespond = PdexV3PriceHistoryRespond{
				Timestamp: tm.Unix(),
				High:      v.High,
				Low:       v.Low,
				Open:      v.Open,
				Close:     v.Close,
			}
			result = append(result, pdexV3PriceHistoryRespond)
		}
	}

	respond := APIRespond{
		Result: result,
	}

	c.JSON(http.StatusOK, respond)
}

func (pdexv3) LiquidityHistory(c *gin.Context) {
	poolid := c.Query("poolid")
	period := c.Query("period")
	intervals := c.Query("intervals")

	analyticsData, err := analyticsquery.APIGetPDexV3PoolLiquidityHistories(poolid, period, intervals)

	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	var result []PdexV3LiquidityHistoryRespond

	for _, v := range analyticsData.Result {
		tm, _ := time.Parse(time.RFC3339, v.Timestamp)

		var pdexV3LiquidityHistoryRespond = PdexV3LiquidityHistoryRespond{
			Timestamp:           tm.Unix(),
			Token0RealAmount:    v.Token0RealAmount,
			Token1RealAmount:    v.Token1RealAmount,
			Token0VirtualAmount: v.Token0VirtualAmount,
			Token1VirtualAmount: v.Token1VirtualAmount,
			ShareAmount:         v.ShareAmount,
		}
		result = append(result, pdexV3LiquidityHistoryRespond)
	}

	respond := APIRespond{
		Result: result,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) TradeVolume24h(c *gin.Context) {
	pair := c.Query("pair")

	_ = pair

	analyticsData, err := analyticsquery.APIGetPDexV3TradingVolume24H(pair)

	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	respond := APIRespond{
		Result: struct {
			Value float64
		}{
			Value: analyticsData.Result.Value,
		},
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) TradeDetail(c *gin.Context) {
	txhash := c.Query("txhash")

	tradeList, err := database.DBGetTxTradeFromTxRequest([]string{txhash})
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	txRequest := []string{txhash}
	tradeStatusList, err := database.DBGetTradeStatus(txRequest)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	var result []TradeDataRespond
	for _, tradeInfo := range tradeList {

		amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)

		if tradeInfo.IsSwap {
			matchedAmount := uint64(0)
			status := ""
			isCompleted := false
			switch tradeInfo.Status {
			case 0:
				status = "pending"
			case 1:
				status = "accepted"
				matchedAmount = amount
				isCompleted = true
			case 2:
				status = "rejected"
				isCompleted = true
			}

			uniqIdx := getUniqueIdx(tradeInfo.RespondTxs)

			trade := TradeDataRespond{
				RequestTx: tradeInfo.RequestTx,
				// RespondTxs:     tradeInfo.RespondTxs,
				// RespondTokens:  tradeInfo.RespondTokens,
				// RespondAmounts: tradeInfo.RespondAmount,
				WithdrawTxs: nil,
				PoolID:      tradeInfo.PoolID,
				PairID:      tradeInfo.PairID,
				SellTokenID: tradeInfo.SellTokenID,
				BuyTokenID:  tradeInfo.BuyTokenID,
				Amount:      amount,
				MinAccept:   minAccept,
				Matched:     matchedAmount,
				Status:      status,
				StatusCode:  tradeInfo.Status,
				Requestime:  tradeInfo.Requesttime,
				NFTID:       tradeInfo.NFTID,
				Fee:         tradeInfo.Fee,
				FeeToken:    tradeInfo.FeeToken,
				Receiver:    tradeInfo.Receiver,
				IsCompleted: isCompleted,
				TradingPath: tradeInfo.TradingPath,
			}
			for _, v := range uniqIdx {
				trade.RespondTxs = append(trade.RespondTxs, tradeInfo.RespondTxs[v])
				trade.RespondTokens = append(trade.RespondTokens, tradeInfo.RespondTokens[v])
				trade.RespondAmounts = append(trade.RespondAmounts, tradeInfo.RespondAmount[v])
			}
			result = append(result, trade)
		} else {
			matchedAmount := uint64(0)
			var tradeStatus *shared.LimitOrderStatus
			if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
				tradeStatus = &t
			}
			matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
			trade := TradeDataRespond{
				RequestTx:           tradeInfo.RequestTx,
				RespondTxs:          tradeInfo.RespondTxs,
				RespondTokens:       tradeInfo.RespondTokens,
				RespondAmounts:      tradeInfo.RespondAmount,
				WithdrawTxs:         withdrawTxs,
				PoolID:              tradeInfo.PoolID,
				PairID:              tradeInfo.PairID,
				SellTokenID:         tradeInfo.SellTokenID,
				BuyTokenID:          tradeInfo.BuyTokenID,
				Amount:              amount,
				MinAccept:           minAccept,
				Matched:             matchedAmount,
				Status:              status,
				StatusCode:          statusCode,
				Requestime:          tradeInfo.Requesttime,
				NFTID:               tradeInfo.NFTID,
				Fee:                 tradeInfo.Fee,
				FeeToken:            tradeInfo.FeeToken,
				Receiver:            tradeInfo.Receiver,
				IsCompleted:         isCompleted,
				SellTokenBalance:    sellTokenBl,
				BuyTokenBalance:     buyTokenBl,
				SellTokenWithdrawed: sellTokenWD,
				BuyTokenWithdrawed:  buyTokenWD,
			}
			result = append(result, trade)
		}

	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) GetRate(c *gin.Context) {
	var req struct {
		TokenIDs []string `json:"TokenIDs"`
		Against  string   `json:"Against"`
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	result := make(map[string]float64)

	state, err := database.DBGetPDEState(2)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	data := `{"Result":` + state + `}`

	var responseBodyData shared.Pdexv3GetStateRPCResult

	err = json.UnmarshalFromString(data, &responseBodyData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	pools, poolPairStates, err := pathfinder.GetPdexv3PoolDataFromRawRPCResult(responseBodyData.Result.Poolpairs)

	if err != nil {
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	for _, v := range req.TokenIDs {
		rt := getRate(req.Against, v, pools, poolPairStates)
		result[v] = rt
	}

	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PendingOrder(c *gin.Context) {
	var result struct {
		Buy  []PdexV3PendingOrderData
		Sell []PdexV3PendingOrderData
	}
	var buyOrders []PdexV3PendingOrderData
	var sellOrders []PdexV3PendingOrderData
	poolID := c.Query("poolid")
	tks := strings.Split(poolID, "-")
	var priorityTokens []string
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
	}

	// pairID := tks[0] + "-" + tks[1]
	list, err := database.DBGetLimitOrderStatusByPoolID(poolID)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	willSwap := willSwapTokenPlace(tks[0], tks[1], priorityTokens)

	var sellSide []shared.LimitOrderStatus
	var buySide []shared.LimitOrderStatus
	txRequestList := []string{}
	orderList := make(map[string]shared.TradeOrderData)
	for _, v := range list {
		if v.Direction == 0 {
			sellSide = append(sellSide, v)
		} else {
			buySide = append(buySide, v)
		}
		txRequestList = append(txRequestList, v.RequestTx)
	}

	l, err := database.DBGetTxTradeFromTxRequest(txRequestList)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	for _, v := range l {
		orderList[v.RequestTx] = v
	}

	for _, v := range sellSide {
		tk1Balance, _ := strconv.ParseUint(v.Token1Balance, 10, 64)
		tk2Balance, _ := strconv.ParseUint(v.Token2Balance, 10, 64)
		if tk1Balance == 0 {
			continue
		}
		tk1Amount, _ := strconv.ParseUint(orderList[v.RequestTx].Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(orderList[v.RequestTx].MinAccept, 10, 64)
		data := PdexV3PendingOrderData{
			TxRequest:    v.RequestTx,
			Token1Remain: tk1Balance,
			Token2Remain: tk2Amount - tk2Balance,
			Token1Amount: tk1Amount,
			Token2Amount: tk2Amount,
		}
		if tk1Amount != 0 {
			data.Rate = float64(tk2Amount) / float64(tk1Amount)
		}
		if willSwap {
			data = PdexV3PendingOrderData{
				TxRequest:    v.RequestTx,
				Token1Remain: tk2Amount - tk2Balance,
				Token2Remain: tk1Balance,
				Token1Amount: tk2Amount,
				Token2Amount: tk1Amount,
			}
			if tk2Amount != 0 {
				data.Rate = float64(tk1Amount) / float64(tk2Amount)
			}
		}
		sellOrders = append(sellOrders, data)
	}
	for _, v := range buySide {
		tk1Balance, _ := strconv.ParseUint(v.Token1Balance, 10, 64)
		tk2Balance, _ := strconv.ParseUint(v.Token2Balance, 10, 64)
		if tk2Balance == 0 {
			continue
		}
		tk2Amount, _ := strconv.ParseUint(orderList[v.RequestTx].Amount, 10, 64)
		tk1Amount, _ := strconv.ParseUint(orderList[v.RequestTx].MinAccept, 10, 64)
		data := PdexV3PendingOrderData{
			TxRequest:    v.RequestTx,
			Token1Remain: tk1Amount - tk1Balance,
			Token2Remain: tk2Balance,
			Token1Amount: tk1Amount,
			Token2Amount: tk2Amount,
		}
		if tk1Amount != 0 {
			data.Rate = float64(tk2Amount) / float64(tk1Amount)
		}
		if willSwap {
			data = PdexV3PendingOrderData{
				TxRequest:    v.RequestTx,
				Token1Remain: tk2Balance,
				Token2Remain: tk1Amount - tk1Balance,
				Token1Amount: tk2Amount,
				Token2Amount: tk1Amount,
			}
			if tk2Amount != 0 {
				data.Rate = float64(tk1Amount) / float64(tk2Amount)
			}
		}
		buyOrders = append(buyOrders, data)
	}

	result.Buy = buyOrders
	result.Sell = sellOrders
	if willSwap {
		result.Buy = sellOrders
		result.Sell = buyOrders
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PDEState(c *gin.Context) {
	state, err := database.DBGetPDEState(2)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	pdeState := jsonresult.Pdexv3State{}
	err = json.UnmarshalFromString(state, &pdeState)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	respond := APIRespond{
		Result: pdeState,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) PendingLimit(c *gin.Context) {
	var req struct {
		Otakey string
		ID     []string
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	nftIDs := req.ID
	rawOTA64 := make(map[string]string)
	if req.Otakey != "" {
		accessOTAList, raw64, err := retrieveAccessOTAList(req.Otakey)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		rawOTA64 = raw64
		nftIDs = accessOTAList
	}

	tradeList, tradeStatus, err := database.DBGetPendingLimitOrderByNftID(nftIDs)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}
	if len(tradeList) == 0 {
		respond := APIRespond{
			Result: []TradeDataRespond{},
			Error:  nil,
		}
		c.JSON(http.StatusOK, respond)
		return
	}

	tradeStatusList := make(map[string]shared.LimitOrderStatus)
	for _, v := range tradeStatus {
		tradeStatusList[v.RequestTx] = v
	}

	var result []TradeDataRespond
	for _, tradeInfo := range tradeList {
		matchedAmount := uint64(0)
		var tradeStatus *shared.LimitOrderStatus
		nextota := ""
		isMinitngNextOTA := false
		if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
			tradeStatus = &t
			if _, ok := rawOTA64[tradeStatus.CurrentAccessID]; ok {
				nextota = rawOTA64[tradeStatus.CurrentAccessID]
			} else {
				isMinitngNextOTA = true
			}
		}
		matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)

		trade := TradeDataRespond{
			RequestTx:             tradeInfo.RequestTx,
			RespondTxs:            tradeInfo.RespondTxs,
			RespondTokens:         tradeInfo.RespondTokens,
			RespondAmounts:        tradeInfo.RespondAmount,
			WithdrawTxs:           withdrawTxs,
			PoolID:                tradeInfo.PoolID,
			PairID:                tradeInfo.PairID,
			SellTokenID:           tradeInfo.SellTokenID,
			BuyTokenID:            tradeInfo.BuyTokenID,
			Amount:                amount,
			MinAccept:             minAccept,
			Matched:               matchedAmount,
			Status:                status,
			StatusCode:            statusCode,
			Requestime:            tradeInfo.Requesttime,
			NFTID:                 tradeInfo.NFTID,
			Fee:                   tradeInfo.Fee,
			FeeToken:              tradeInfo.FeeToken,
			Receiver:              tradeInfo.Receiver,
			IsCompleted:           isCompleted,
			SellTokenBalance:      sellTokenBl,
			BuyTokenBalance:       buyTokenBl,
			SellTokenWithdrawed:   sellTokenWD,
			BuyTokenWithdrawed:    buyTokenWD,
			CurrentAccessOTA:      nextota,
			IsMintingNewAccessOTA: isMinitngNextOTA,
		}
		result = append(result, trade)
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) ListMarkets(c *gin.Context) {
	datalist := getMarketTokenList()
	respond := APIRespond{
		Result: datalist,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) GetAccessOTAData(c *gin.Context) {
	var req struct {
		ID             []string
		GetOrder       bool
		GetShare       bool
		GetOrderReward bool
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, buildGinErrorRespond(err))
		return
	}

	var otas []string
	rawOTA64 := make(map[string]string)
	orders := make(map[string]shared.TradeOrderData)
	shares := make(map[string]PdexV3PoolShareRespond)
	orderRewards := make(map[string]PdexV3PoolShareRespond)

	for _, v := range req.ID {
		otaBytes, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		ota := common.HashH(otaBytes[:]).String()
		otas = append(otas, ota)
		rawOTA64[ota] = v
	}
	if req.GetOrder {
		ordersList, err := database.DBGetPendingLimitOrderByAccessOTA(otas)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}
		for k, v := range ordersList {
			accessB64 := rawOTA64[k]
			orders[accessB64] = v
		}
	}
	if req.GetShare {
		list, err := database.DBGetShareByCurrentAccessID(otas)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}

		sharesList, err := producePoolShareRespond(list, true, rawOTA64)
		if err != nil {
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
		}
		for _, v := range sharesList {
			shares[v.CurrentAccessOTA] = v
		}
	}

	if req.GetOrderReward {
		list, err := database.DBGetShare(otas)
		if err != nil {
			c.JSON(http.StatusOK, buildGinErrorRespond(err))
			return
		}

		ordersList, err := producePoolShareRespond(list, true, rawOTA64)
		if err != nil {
			if err != nil {
				c.JSON(http.StatusOK, buildGinErrorRespond(err))
				return
			}
		}
		for _, v := range ordersList {
			shares[v.CurrentAccessOTA] = v
		}
	}

	result := InUseAccessOTAData{
		Orders:       orders,
		Shares:       shares,
		OrderRewards: orderRewards,
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
