package apiservice

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/pdexv3/feeestimator"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/metadata"
	pdexv3Meta "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

type pdexv3 struct{}

func (pdexv3) ListPairs(c *gin.Context) {
	var result []PdexV3PairData
	list, err := database.DBGetPdexPairs()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
	isverify := false
	if verify == "true" {
		isverify = true
	}
	if pair == "all" {
		var result []PdexV3PoolDetail
		err := cacheGet("ListPools-all", &result)
		if err != nil {
			fmt.Println("cacheStoreCustom failed")
			log.Println(err)
		} else {
			fmt.Println("cacheStoreCustom success 1111")
			respond := APIRespond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
			return
		}
	}
	list, err := database.DBGetPoolPairsByPairID(pair)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var defaultPools map[string]struct{}
	var priorityTokens []string
	if err := cacheGet(defaultPoolsKey, &defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool(true)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(defaultPoolsKey, defaultPools)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	// Get pool pair rate changes
	poolIds := make([]string, 0)
	for _, v := range list {
		poolIds = append(poolIds, v.PoolID)
	}
	poolLiquidityChanges, err := analyticsquery.APIGetPDexV3PairRateChangesAndVolume24h(poolIds)

	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	var result []PdexV3PoolDetail
	var wg sync.WaitGroup
	wg.Add(len(list))
	resultCh := make(chan PdexV3PoolDetail, len(list))
	for _, v := range list {
		go func(d shared.PoolInfoData) {
			var data *PdexV3PoolDetail
			defer func() {
				wg.Done()
				if data != nil {
					resultCh <- *data
				}
			}()
			tk1Amount, _ := strconv.ParseUint(d.Token1Amount, 10, 64)
			tk2Amount, _ := strconv.ParseUint(d.Token2Amount, 10, 64)
			if tk1Amount == 0 || tk2Amount == 0 {
				return
			}
			dcrate, _, _, err := getPdecimalRate(d.TokenID1, d.TokenID2)
			if err != nil {
				log.Println(err)
				return
			}
			token1ID := d.TokenID1
			token2ID := d.TokenID2
			tk1VA, _ := strconv.ParseUint(d.Virtual1Amount, 10, 64)
			tk2VA, _ := strconv.ParseUint(d.Virtual2Amount, 10, 64)
			totalShare, _ := strconv.ParseUint(d.TotalShare, 10, 64)

			willSwap := willSwapTokenPlace(token1ID, token2ID, priorityTokens)
			if willSwap {
				token1ID = d.TokenID2
				token2ID = d.TokenID1
				tk1VA, _ = strconv.ParseUint(d.Virtual2Amount, 10, 64)
				tk2VA, _ = strconv.ParseUint(d.Virtual1Amount, 10, 64)
				tk1Amount, _ = strconv.ParseUint(d.Token2Amount, 10, 64)
				tk2Amount, _ = strconv.ParseUint(d.Token1Amount, 10, 64)
				dcrate, _, _, err = getPdecimalRate(d.TokenID2, d.TokenID1)
				if err != nil {
					log.Println(err)
					return
				}
			}

			data = &PdexV3PoolDetail{
				PoolID:         d.PoolID,
				Token1ID:       token1ID,
				Token2ID:       token2ID,
				Token1Value:    tk1Amount,
				Token2Value:    tk2Amount,
				Virtual1Value:  tk1VA,
				Virtual2Value:  tk2VA,
				Volume:         0,
				PriceChange24h: 0,
				AMP:            d.AMP,
				Price:          calcRateSimple(float64(tk1VA), float64(tk2VA)) * dcrate,
				TotalShare:     totalShare,
			}

			if poolChange, found := poolLiquidityChanges[d.PoolID]; found {
				data.PriceChange24h = poolChange.RateChangePercentage
				data.Volume = poolChange.TradingVolume24h
			}

			apy, err := database.DBGetPDEPoolPairRewardAPY(data.PoolID)
			if err != nil {
				log.Println(err)
				return
			}
			if apy != nil {
				data.APY = uint64(apy.APY2)
			}
			if _, found := defaultPools[d.PoolID]; found {
				data.IsVerify = true
			}
			if isverify && !data.IsVerify {
				data = nil
			}
		}(v)
	}
	wg.Wait()
	close(resultCh)
	for v := range resultCh {
		result = append(result, v)
	}
	go cacheStoreCustom("ListPools-all", result, 10*time.Second)
	fmt.Println("cacheStoreCustom success")
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
	list, err := database.DBGetShare(accessID)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	var result []PdexV3PoolShareRespond
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	// err = cacheStore(tokenPriorityKey, priorityTokens)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
	// 	return
	// }
	for _, v := range list {
		l, err := database.DBGetPoolPairsByPoolID([]string{v.PoolID})
		if err != nil {
			errStr := err.Error()
			respond := APIRespond{
				Result: nil,
				Error:  &errStr,
			}
			c.JSON(http.StatusOK, respond)
			return
		}
		if len(l) == 0 {
			continue
		}
		if (v.Amount == 0) && len(v.TradingFee) == 0 && len(v.OrderReward) == 0 {
			continue
		}
		token1ID := l[0].TokenID1
		token2ID := l[0].TokenID2
		tk1Amount, _ := strconv.ParseUint(l[0].Token1Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(l[0].Token2Amount, 10, 64)
		totalShare, _ := strconv.ParseUint(l[0].TotalShare, 10, 64)

		willSwap := willSwapTokenPlace(token1ID, token2ID, priorityTokens)
		if willSwap {
			token1ID = l[0].TokenID2
			token2ID = l[0].TokenID1
			tk1Amount, _ = strconv.ParseUint(l[0].Token2Amount, 10, 64)
			tk2Amount, _ = strconv.ParseUint(l[0].Token1Amount, 10, 64)
		}

		result = append(result, PdexV3PoolShareRespond{
			PoolID:       v.PoolID,
			Share:        v.Amount,
			Rewards:      v.TradingFee,
			AMP:          l[0].AMP,
			TokenID1:     token1ID,
			TokenID2:     token2ID,
			Token1Amount: tk1Amount,
			Token2Amount: tk2Amount,
			TotalShare:   totalShare,
			OrderRewards: v.OrderReward,
		})
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
	otakey := c.Query("otakey")
	poolid := c.Query("poolid")
	nftID := c.Query("nftid")
	isOrder := c.Query("isorder")
	getOrder := false
	if nftID == "" {
		errStr := "nftID can't be empty"
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

	if otakey != "" {
		var result []TradeDataRespond
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
			wl, err := wallet.Base58CheckDeserialize(otakey)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("invalid otakey")))
				return
			}
			coinList, err := database.DBGetAllAccessCoin(base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()))
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			for _, v := range coinList {
				coinBytes, _, err := base58.Base58Check{}.Decode(v.CoinPubkey)
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
					return
				}
				accessID := common.Hash{}
				err = accessID.SetBytes(coinBytes)
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
					return
				}
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
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			txRequest := []string{}
			for _, tx := range txList {
				txRequest = append(txRequest, tx.TxHash)
			}
			list, err := database.DBGetTxTradeFromTxRequest(txRequest)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
					RequestTx: tradeInfo.RequestTx,
					// RespondTxs: tradeInfo.RespondTxs,
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
			}
		}

		respond := APIRespond{
			Result: result,
			Error:  nil,
		}
		log.Println("APIGetTradeHistory time:", time.Since(startTime))
		c.JSON(http.StatusOK, respond)
	} else {
		//limit order
		tradeList, err := database.DBGetTxTradeFromPoolAndNFT(poolid, nftID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		txRequest := []string{}
		for _, tx := range tradeList {
			txRequest = append(txRequest, tx.RequestTx)
		}
		tradeStatusList, err := database.DBGetTradeStatus(txRequest)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		var result []TradeDataRespond
		for _, tradeInfo := range tradeList {
			matchedAmount := uint64(0)
			var tradeStatus *shared.LimitOrderStatus
			if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
				tradeStatus = &t
			}
			matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
			minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
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
		respond := APIRespond{
			Result: result,
			Error:  nil,
		}
		log.Println("APIGetTradeHistory time:", time.Since(startTime))
		c.JSON(http.StatusOK, respond)
	}
}

func (pdexv3) ContributeHistory(c *gin.Context) {
	offset, _ := strconv.Atoi(c.Query("offset"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	poolID := c.Query("poolid")
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

	var err error
	var list []shared.ContributionData
	if poolID != "" {

	} else {
		list, err = database.DBGetPDEV3ContributeRespond(accessID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}

	var result []PdexV3ContributionData

	for _, v := range list {
		ctrbAmount := []uint64{}
		ctrbToken := []string{}
		if len(v.RequestTxs) > len(v.ContributeAmount) {
			a, _ := strconv.ParseUint(v.ContributeAmount[0], 10, 64)
			ctrbAmount = append(ctrbAmount, a)
			ctrbAmount = append(ctrbAmount, a)
		} else {
			for _, v := range v.ContributeAmount {
				a, _ := strconv.ParseUint(v, 10, 64)
				ctrbAmount = append(ctrbAmount, a)
			}

		}
		if len(v.RequestTxs) > len(v.ContributeTokens) {
			ctrbToken = append(ctrbToken, v.ContributeTokens[0])
			ctrbToken = append(ctrbToken, v.ContributeTokens[0])
		} else {
			ctrbToken = v.ContributeTokens
		}
		returnAmount := []uint64{}
		for _, v := range v.ReturnAmount {
			a, _ := strconv.ParseUint(v, 10, 64)
			returnAmount = append(returnAmount, a)
		}

		data := PdexV3ContributionData{
			RequestTxs:       v.RequestTxs,
			RespondTxs:       v.RespondTxs,
			ContributeTokens: ctrbToken,
			ContributeAmount: ctrbAmount,
			PairID:           v.PairID,
			PairHash:         v.PairHash,
			ReturnTokens:     v.ReturnTokens,
			ReturnAmount:     returnAmount,
			NFTID:            v.NFTID,
			RequestTime:      v.RequestTime,
			PoolID:           v.PoolID,
			Status:           "waiting",
		}
		if len(v.RequestTxs) == 2 && len(v.RespondTxs) == 0 {
			if v.ContributeTokens[0] != v.ContributeTokens[1] {
				data.Status = "completed"
			} else {
				data.Status = "refunding"
			}
		}
		if len(v.RespondTxs) > 0 {
			data.Status = "refunded"
		}
		result = append(result, data)
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
	var result []PdexV3WithdrawRespond
	var err error
	var list []shared.WithdrawContributionData

	if poolID != "" {
		list, err = database.DBGetPDEV3WithdrawRespond(accessID, poolID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	} else {
		list, err = database.DBGetPDEV3WithdrawRespond(accessID, "", int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}

	for _, v := range list {
		var token1, token2 string
		var amount1, amount2 uint64
		if len(v.RespondTxs) == 2 {
			amount1, _ = strconv.ParseUint(v.WithdrawAmount[0], 10, 64)
			amount2, _ = strconv.ParseUint(v.WithdrawAmount[1], 10, 64)
		}
		if len(v.RespondTxs) == 1 {
			amount1, _ = strconv.ParseUint(v.WithdrawAmount[0], 10, 64)
		}
		tks := strings.Split(v.PoolID, "-")
		token1 = tks[0]
		token2 = tks[1]
		shareAmount, err := strconv.ParseUint(v.ShareAmount, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		result = append(result, PdexV3WithdrawRespond{
			PoolID:      v.PoolID,
			RequestTx:   v.RequestTx,
			RespondTxs:  v.RespondTxs,
			TokenID1:    token1,
			Amount1:     amount1,
			TokenID2:    token2,
			Amount2:     amount2,
			Status:      v.Status,
			ShareAmount: shareAmount,
			Requestime:  v.RequestTime,
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
	poolID := c.Query("poolid")
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
	var result []PdexV3WithdrawFeeRespond
	var err error
	var list []shared.WithdrawContributionFeeData

	if poolID != "" {
		list, err = database.DBGetPDEV3WithdrawFeeRespond(accessID, poolID, int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	} else {
		list, err = database.DBGetPDEV3WithdrawFeeRespond(accessID, "", int64(limit), int64(offset))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	var result []PdexV3StakingPoolInfo
	for _, v := range list {

		apy, err := database.DBGetPDEPoolPairRewardAPY(v.TokenID)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	list, err := database.DBGetPairsByID(req.PairIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	list, err := database.DBGetPoolPairsByPoolID(req.PoolIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var defaultPools map[string]struct{}
	var priorityTokens []string
	if err := cacheGet(defaultPoolsKey, &defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool(true)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(defaultPoolsKey, defaultPools)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}

	poolLiquidityChanges, err := analyticsquery.APIGetPDexV3PairRateChangesAndVolume24h(req.PoolIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	var result []PdexV3PoolDetail
	for _, v := range list {
		tk1Amount, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
		if tk1Amount == 0 || tk2Amount == 0 {
			continue
		}
		dcrate, _, _, err := getPdecimalRate(v.TokenID1, v.TokenID2)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		token1ID := v.TokenID1
		token2ID := v.TokenID2
		tk1VA, _ := strconv.ParseUint(v.Virtual1Amount, 10, 64)
		tk2VA, _ := strconv.ParseUint(v.Virtual2Amount, 10, 64)
		totalShare, _ := strconv.ParseUint(v.TotalShare, 10, 64)

		willSwap := willSwapTokenPlace(token1ID, token2ID, priorityTokens)
		if willSwap {
			token1ID = v.TokenID2
			token2ID = v.TokenID1
			tk1VA, _ = strconv.ParseUint(v.Virtual2Amount, 10, 64)
			tk2VA, _ = strconv.ParseUint(v.Virtual1Amount, 10, 64)
			tk1Amount, _ = strconv.ParseUint(v.Token2Amount, 10, 64)
			tk2Amount, _ = strconv.ParseUint(v.Token1Amount, 10, 64)
			dcrate, _, _, err = getPdecimalRate(v.TokenID2, v.TokenID1)
			if err != nil {
				log.Println(err)
				return
			}
		}

		data := PdexV3PoolDetail{
			PoolID:         v.PoolID,
			Token1ID:       token1ID,
			Token2ID:       token2ID,
			Token1Value:    tk1Amount,
			Token2Value:    tk2Amount,
			Virtual1Value:  tk1VA,
			Virtual2Value:  tk2VA,
			PriceChange24h: 0,
			Volume:         0,
			AMP:            v.AMP,
			Price:          calcRateSimple(float64(tk1VA), float64(tk2VA)) * dcrate,
			TotalShare:     totalShare,
		}

		if poolChange, found := poolLiquidityChanges[v.PoolID]; found {
			data.PriceChange24h = poolChange.RateChangePercentage
			data.Volume = poolChange.TradingVolume24h
		}

		apy, err := database.DBGetPDEPoolPairRewardAPY(data.PoolID)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if apy != nil {
			data.APY = uint64(apy.APY2)
		}
		if _, found := defaultPools[v.PoolID]; found {
			data.IsVerify = true
		}
		result = append(result, data)
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

	decimalFloat, err := strconv.ParseFloat(decimal, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var priorityTokens []string
	if err := cacheGet(tokenPriorityKey, &priorityTokens); err != nil {
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}
	tks := strings.Split(poolID, "-")
	pairID := tks[0] + "-" + tks[1]
	list, err := database.DBGetPendingOrderByPairID(pairID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	sellToken := req.SellToken
	buyToken := req.BuyToken
	// feeInPRV := req.FeeInPRV
	sellAmount := req.SellAmount
	buyAmount := req.BuyAmount

	if sellAmount <= 0 && buyAmount <= 0 {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("sellAmount and buyAmount must be >=0")))
		return
	}
	if sellAmount > 0 && buyAmount > 0 {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(errors.New("only accept sellAmount or buyAmount value individually")))
		return
	}

	var result PdexV3EstimateTradeRespondBig
	var feePRV PdexV3EstimateTradeRespond
	var feeToken PdexV3EstimateTradeRespond

	state, err := database.DBGetPDEState(2)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(defaultPoolsKey, defaultPools)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		ia := ((rt1 / rt) - 1) * 100
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
		ia := ((rt1 / rt) - 1) * 100
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
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	txRequest := []string{txhash}
	tradeStatusList, err := database.DBGetTradeStatus(txRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	result := make(map[string]float64)

	state, err := database.DBGetPDEState(2)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		err = cacheStore(tokenPriorityKey, priorityTokens)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
	}

	pairID := tks[0] + "-" + tks[1]
	list, err := database.DBGetLimitOrderStatusByPairID(pairID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
			Rate:         float64(tk2Amount) / float64(tk1Amount),
		}
		if willSwap {
			data = PdexV3PendingOrderData{
				TxRequest:    v.RequestTx,
				Token1Remain: tk2Amount - tk2Balance,
				Token2Remain: tk1Balance,
				Token1Amount: tk2Amount,
				Token2Amount: tk1Amount,
				Rate:         float64(tk1Amount) / float64(tk2Amount),
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
			Rate:         float64(tk2Amount) / float64(tk1Amount),
		}
		if willSwap {
			data = PdexV3PendingOrderData{
				TxRequest:    v.RequestTx,
				Token1Remain: tk2Balance,
				Token2Remain: tk1Amount - tk1Balance,
				Token1Amount: tk2Amount,
				Token2Amount: tk1Amount,
				Rate:         float64(tk1Amount) / float64(tk2Amount),
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
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	pdeState := jsonresult.Pdexv3State{}
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

func (pdexv3) PendingLimit(c *gin.Context) {
	var req struct {
		ID []string
	}
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}

	tradeList, err := database.DBGetPendingLimitOrderByNftID(req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
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
	txRequest := []string{}
	for _, tx := range tradeList {
		txRequest = append(txRequest, tx.RequestTx)
	}
	tradeStatusList, err := database.DBGetTradeStatus(txRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var result []TradeDataRespond
	for _, tradeInfo := range tradeList {
		matchedAmount := uint64(0)
		var tradeStatus *shared.LimitOrderStatus
		if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
			tradeStatus = &t
		}
		matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		amount, _ := strconv.ParseUint(tradeInfo.Amount, 10, 64)
		minAccept, _ := strconv.ParseUint(tradeInfo.MinAccept, 10, 64)
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
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func (pdexv3) ListMarkets(c *gin.Context) {
	var datalist []TokenInfo

	defaultPools, err := database.DBGetDefaultPool(true)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	tokenMap := make(map[string]struct{})
	tokens := []string{}
	for poolID := range defaultPools {
		tks := strings.Split(poolID, "-")
		tokenMap[tks[0]] = struct{}{}
		tokenMap[tks[1]] = struct{}{}
	}
	for v := range tokenMap {
		tokens = append(tokens, v)
	}
	extraTokenInfo, err := database.DBGetExtraTokenInfoByTokenID(tokens)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}
	customTokenInfo, err := database.DBGetCustomTokenInfoByTokenID(tokens)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	extraTokenInfoMap := make(map[string]shared.ExtraTokenInfo)
	for _, v := range extraTokenInfo {
		extraTokenInfoMap[v.TokenID] = v
	}

	customTokenInfoMap := make(map[string]shared.CustomTokenInfo)
	for _, v := range customTokenInfo {
		customTokenInfoMap[v.TokenID] = v
	}
	chainTkListMap := make(map[string]struct{})

	baseToken, _ := database.DBGetBasePriceToken()

	prvUsdtPair24h := float64(0)
	for v, _ := range defaultPools {
		if strings.Contains(v, baseToken) && strings.Contains(v, common.PRVCoinID.String()) {
			prvUsdtPair24h = getPoolPair24hChange(v)
			break
		}
	}

	tokenList, err := database.DBGetTokenByTokenID(tokens)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, buildGinErrorRespond(err))
		return
	}

	for _, v := range tokenList {
		chainTkListMap[v.TokenID] = struct{}{}
		currPrice, _ := strconv.ParseFloat(v.CurrentPrice, 64)
		pastPrice, _ := strconv.ParseFloat(v.PastPrice, 64)
		percent24h := float64(0)
		if pastPrice != 0 && currPrice != 0 {
			percent24h = ((currPrice - pastPrice) / pastPrice) * 100
		}
		data := TokenInfo{
			TokenID:          v.TokenID,
			Name:             v.Name,
			Symbol:           v.Symbol,
			Image:            v.Image,
			IsPrivacy:        v.IsPrivacy,
			IsBridge:         v.IsBridge,
			ExternalID:       v.ExternalID,
			PriceUsd:         currPrice,
			PercentChange24h: fmt.Sprintf("%.2f", percent24h),
		}

		defaultPool := ""
		defaultPairToken := ""
		defaultPairTokenIdx := -1
		currentPoolAmount := uint64(0)
		for poolID, _ := range defaultPools {
			if strings.Contains(poolID, data.TokenID) {
				pa := getPoolAmount(poolID, data.TokenID)
				if pa == 0 {
					continue
				}
				tks := strings.Split(poolID, "-")
				tkPair := tks[0]
				if tks[0] == data.TokenID {
					tkPair = tks[1]
				}
				for idx, ptk := range priorityTokens {
					if (ptk == tkPair) && (idx >= defaultPairTokenIdx) {
						if idx > defaultPairTokenIdx {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
						if (idx == defaultPairTokenIdx) && (pa > currentPoolAmount) {
							defaultPool = poolID
							defaultPairToken = tkPair
							defaultPairTokenIdx = idx
							currentPoolAmount = pa
						}
					}
				}

				if defaultPool == "" {
					if pa > 0 {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				} else {
					if (pa > currentPoolAmount) && (defaultPairTokenIdx == -1) {
						defaultPool = poolID
						defaultPairToken = tkPair
						currentPoolAmount = pa
					}
				}
			}
		}
		data.DefaultPairToken = defaultPairToken
		data.DefaultPoolPair = defaultPool
		if data.TokenID == common.PRVCoinID.String() {
			data.PercentChange24h = fmt.Sprintf("%.2f", prvUsdtPair24h)
		} else {
			if data.DefaultPairToken != "" && data.TokenID != baseToken {
				data.PercentChange24h = fmt.Sprintf("%.2f", getToken24hPriceChange(data.TokenID, data.DefaultPairToken, data.DefaultPoolPair, baseToken, prvUsdtPair24h))
			}
		}

		if etki, ok := customTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			if etki.Verified {
				data.Verified = etki.Verified
			}
		}
		if etki, ok := extraTokenInfoMap[v.TokenID]; ok {
			if etki.Name != "" {
				data.Name = etki.Name
			}
			data.Decimals = etki.Decimals
			if etki.Symbol != "" {
				data.Symbol = etki.Symbol
			}
			data.PSymbol = etki.PSymbol
			data.PDecimals = int(etki.PDecimals)
			data.ContractID = etki.ContractID
			data.Status = etki.Status
			data.Type = etki.Type
			data.CurrencyType = etki.CurrencyType
			data.Default = etki.Default
			if etki.Verified {
				data.Verified = etki.Verified
			}
			data.UserID = etki.UserID
			data.PercentChange1h = etki.PercentChange1h
			data.PercentChangePrv1h = etki.PercentChangePrv1h
			data.CurrentPrvPool = etki.CurrentPrvPool
			data.PricePrv = etki.PricePrv
			data.Volume24 = etki.Volume24
			data.ParentID = etki.ParentID
			data.OriginalSymbol = etki.OriginalSymbol
			data.LiquidityReward = etki.LiquidityReward
			data.Network = etki.Network
			err = json.UnmarshalFromString(etki.ListChildToken, &data.ListChildToken)
			if err != nil {
				panic(err)
			}
			if data.PriceUsd == 0 {
				data.PriceUsd = etki.PriceUsd
			}

		}

		if !v.IsNFT {
			datalist = append(datalist, data)
		}
	}

	for _, tkInfo := range extraTokenInfo {
		if _, ok := chainTkListMap[tkInfo.TokenID]; !ok {
			tkdata := TokenInfo{
				TokenID:      tkInfo.TokenID,
				Name:         tkInfo.Name,
				Symbol:       tkInfo.Symbol,
				PSymbol:      tkInfo.PSymbol,
				PDecimals:    int(tkInfo.PDecimals),
				Decimals:     tkInfo.Decimals,
				ContractID:   tkInfo.ContractID,
				Status:       tkInfo.Status,
				Type:         tkInfo.Type,
				CurrencyType: tkInfo.CurrencyType,
				Default:      tkInfo.Default,
				Verified:     tkInfo.Verified,
				UserID:       tkInfo.UserID,

				PriceUsd:           tkInfo.PriceUsd,
				PercentChange1h:    tkInfo.PercentChange1h,
				PercentChangePrv1h: tkInfo.PercentChangePrv1h,
				CurrentPrvPool:     tkInfo.CurrentPrvPool,
				PricePrv:           tkInfo.PricePrv,
				Volume24:           tkInfo.Volume24,
				ParentID:           tkInfo.ParentID,
				OriginalSymbol:     tkInfo.OriginalSymbol,
				LiquidityReward:    tkInfo.LiquidityReward,

				Network: tkInfo.Network,
			}
			err = json.UnmarshalFromString(tkInfo.ListChildToken, &tkdata.ListChildToken)
			if err != nil {
				panic(err)
			}
			datalist = append(datalist, tkdata)
		}
	}
	respond := APIRespond{
		Result: datalist,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
