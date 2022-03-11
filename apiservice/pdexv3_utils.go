package apiservice

import (
	"errors"
	"strconv"
	"strings"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/wallet"
)

func produceTradeDataRespond(tradeList []shared.TradeOrderData, tradeStatusList map[string]shared.LimitOrderStatus) ([]TradeDataRespond, error) {
	var result []TradeDataRespond
	for _, tradeInfo := range tradeList {
		matchedAmount := uint64(0)
		var tradeStatus *shared.LimitOrderStatus
		if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
			tradeStatus = &t
		}
		matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
		if err != nil {
			return nil, err
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
	return result, nil
}

func produceWithdrawContributeData(list []shared.WithdrawContributionData) ([]PdexV3WithdrawRespond, error) {
	var result []PdexV3WithdrawRespond
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
			return nil, err
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
	return result, nil
}

func produceContributeData(list []shared.ContributionData) ([]PdexV3ContributionData, error) {
	var result []PdexV3ContributionData
	var contributeList []PdexV3ContributionData
	completedTxs := make(map[string]struct{})
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
			AccessIDs:        v.AccessIDs,
			Status:           "waiting",
		}
		if len(v.RequestTxs) == 2 && len(v.RespondTxs) == 0 {
			if v.ContributeTokens[0] != v.ContributeTokens[1] {
				data.Status = "completed"
			} else {
				data.Status = "refunding"
			}
		}
		if len(v.RequestTxs) == 2 {
			for _, tx := range v.RequestTxs {
				completedTxs[tx] = struct{}{}
			}
		}
		if len(v.RespondTxs) > 0 {
			data.Status = "refunded"
		}
		contributeList = append(contributeList, data)
	}

	//remove unneeded data

	alreadyAdded := make(map[string]struct{})
	for _, v := range contributeList {
		if len(v.RequestTxs) == 1 {
			if _, ok := completedTxs[v.RequestTxs[0]]; !ok {
				result = append(result, v)
			}
		}
		if len(v.RequestTxs) == 2 {
			_, ok1 := alreadyAdded[v.RequestTxs[0]]
			_, ok2 := alreadyAdded[v.RequestTxs[1]]
			if !ok1 && !ok2 {
				result = append(result, v)
				alreadyAdded[v.RequestTxs[0]] = struct{}{}
				alreadyAdded[v.RequestTxs[1]] = struct{}{}
			}
		}
	}
	return result, nil
}

func producePoolShareRespond(list []shared.PoolShareData) ([]PdexV3PoolShareRespond, error) {
	var result []PdexV3PoolShareRespond
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		l, err := database.DBGetPoolPairsByPoolID([]string{v.PoolID})
		if err != nil {
			return nil, err
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
			PoolID:          v.PoolID,
			Share:           v.Amount,
			Rewards:         v.TradingFee,
			AMP:             l[0].AMP,
			TokenID1:        token1ID,
			TokenID2:        token2ID,
			Token1Amount:    tk1Amount,
			Token2Amount:    tk2Amount,
			TotalShare:      totalShare,
			OrderRewards:    v.OrderReward,
			CurrentAccessID: v.CurrentAccessID,
			NFTID:           v.NFTID,
		})
	}
	return result, nil
}

func retrieveAccessOTAList(otakey string) ([]string, error) {
	var result []string
	wl, err := wallet.Base58CheckDeserialize(otakey)
	if err != nil {
		return nil, err
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		return nil, errors.New("invalid otakey")
	}
	coinList, err := database.DBGetAllAccessCoin(base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()))
	if err != nil {
		return nil, err
	}
	for _, v := range coinList {
		coinBytes, _, err := base58.Base58Check{}.Decode(v.CoinPubkey)
		if err != nil {

			return nil, err
		}
		// accessID := common.Hash{}
		// err = accessID.SetBytes(coinBytes)
		// if err != nil {
		// 	c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		// 	return
		// }
		currentAccess := common.HashH(coinBytes[:]).String()
		result = append(result, currentAccess)
	}
	return result, nil
}
