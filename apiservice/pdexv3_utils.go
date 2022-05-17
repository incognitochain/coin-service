package apiservice

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/key"
	"github.com/incognitochain/incognito-chain/privacy/operation"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

func produceTradeDataRespond(tradeList []shared.TradeOrderData, tradeStatusList map[string]shared.LimitOrderStatus, rawOTA64 map[string]string) ([]TradeDataRespond, error) {
	var result []TradeDataRespond
	for _, tradeInfo := range tradeList {
		matchedAmount := uint64(0)
		var tradeStatus *shared.LimitOrderStatus
		nextota := ""
		isMinitngNextOTA := false
		if t, ok := tradeStatusList[tradeInfo.RequestTx]; ok {
			tradeStatus = &t
			if tradeStatus.CurrentAccessID != "" {
				if _, ok := rawOTA64[tradeStatus.CurrentAccessID]; ok {
					nextota = rawOTA64[tradeStatus.CurrentAccessID]
				} else {
					isMinitngNextOTA = true
				}
			}
		}
		matchedAmount, sellTokenBl, buyTokenBl, sellTokenWD, buyTokenWD, statusCode, status, withdrawTxs, isCompleted, err := getTradeStatus(&tradeInfo, tradeStatus)
		if err != nil {
			return nil, err
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

func produceContributeData(list []shared.ContributionData, isNextOTA bool, rawOTA64 map[string]string) ([]PdexV3ContributionData, error) {
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
			Status:           "waiting",
		}
		if isNextOTA {
			for _, v := range v.AccessIDs {
				if ota, ok := rawOTA64[v]; ok {
					data.AccessIDs = append(data.AccessIDs, ota)
				}
			}
			if len(data.AccessIDs) == 0 {
				if len(v.RequestTxs) == 1 {
					data.AccessIDs = v.AccessIDs
				}
			}
		}
		if len(v.RequestTxs) == 2 && len(v.RespondTxs) == 0 {
			if v.ContributeTokens[0] != v.ContributeTokens[1] {
				data.Status = "completed"
			} else {
				data.Status = "waiting"
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

func producePoolShareRespond(list []shared.PoolShareData, isNextOTA bool, rawOTA64 map[string]string) ([]PdexV3PoolShareRespond, error) {
	var result []PdexV3PoolShareRespond
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		return nil, err
	}
	processedNextOTA := make(map[string]struct{})
	for _, v := range list {
		l, err := database.DBGetPoolPairsByPoolID([]string{v.PoolID})
		if err != nil {
			return nil, err
		}
		if len(l) == 0 {
			continue
		}
		if v.Amount == 0 {
			totalFee := uint64(0)
			totalReward := uint64(0)
			for _, v := range v.TradingFee {
				totalFee += v
			}
			for _, v := range v.OrderReward {
				totalReward += v
			}
			if totalFee == 0 && totalReward == 0 {
				continue
			}
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
		nextota := ""
		isMinitngNextOTA := false
		if v.CurrentAccessID != "" {
			if _, ok := processedNextOTA[v.CurrentAccessID]; !ok {
				if _, ok1 := rawOTA64[v.CurrentAccessID]; ok1 {
					nextota = rawOTA64[v.CurrentAccessID]
					processedNextOTA[v.CurrentAccessID] = struct{}{}
				} else {
					isMinitngNextOTA = true
				}
			} else {
				continue
			}
		}
		result = append(result, PdexV3PoolShareRespond{
			PoolID:                v.PoolID,
			Share:                 v.Amount,
			Rewards:               v.TradingFee,
			AMP:                   l[0].AMP,
			TokenID1:              token1ID,
			TokenID2:              token2ID,
			Token1Amount:          tk1Amount,
			Token2Amount:          tk2Amount,
			TotalShare:            totalShare,
			OrderRewards:          v.OrderReward,
			CurrentAccessOTA:      nextota,
			IsMintingNewAccessOTA: isMinitngNextOTA,
			NFTID:                 v.NFTID,
		})
	}
	return result, nil
}

func retrieveAccessOTAList(otakey string) ([]string, map[string]string, error) {
	var result []string
	wl, err := wallet.Base58CheckDeserialize(otakey)
	if err != nil {
		return nil, nil, err
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		return nil, nil, errors.New("invalid otakey")
	}
	coinList, err := database.DBGetAllAccessCoin(base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()))
	if err != nil {
		return nil, nil, err
	}
	rawBase64 := make(map[string]string)
	for _, v := range coinList {
		coinBytes, _, err := base58.Base58Check{}.Decode(v.CoinPubkey)
		if err != nil {

			return nil, nil, err
		}
		currentAccess := common.HashH(coinBytes[:]).String()
		rawBase64[currentAccess] = base64.StdEncoding.EncodeToString(coinBytes[:])
		result = append(result, currentAccess)
	}
	return result, rawBase64, nil
}

var preloadPdexStateV3 PreLoadPdexStatev3

func prefetchPdexV3State() {
	preloadPdexStateV3.WaitingContributions.cacheOTAList = map[string][]string{}
	for {
		state, err := database.DBGetPDEState(2)
		if err != nil {
			panic(err)
		}
		var pdeState jsonresult.Pdexv3State
		err = json.UnmarshalFromString(state, &pdeState)
		if err != nil {
			panic(err)
		}
		if pdeState.BeaconTimeStamp != preloadPdexStateV3.BeaconTimestamp {
			preloadPdexStateV3.PdexRaw = state
			preloadPdexStateV3.WaitingContributions.Lock()
			wcontrbList, accessOTAList, err := processWaitingContributions(pdeState.WaitingContributions)
			if err != nil {
				panic(err)
			}
			preloadPdexStateV3.WaitingContributions.List = wcontrbList
			preloadPdexStateV3.WaitingContributions.AccessOTAs = accessOTAList
			preloadPdexStateV3.WaitingContributions.cacheOTAList = make(map[string][]string)
			preloadPdexStateV3.WaitingContributions.Unlock()

		}
		time.Sleep(10 * time.Second)
	}
}

func getPdexStateV3() *jsonresult.Pdexv3State {
	if preloadPdexStateV3.PdexRaw != "" {
		var pdeState jsonresult.Pdexv3State
		err := json.UnmarshalFromString(preloadPdexStateV3.PdexRaw, &pdeState)
		if err != nil {
			log.Println(err)
			return nil
		}
		return &pdeState
	}
	return nil
}

func checkAccessOTABelongToKey(accessOTA *privacy.OTAReceiver, otakey key.OTAKey) (bool, error) {
	txOTARandomPoint, err := accessOTA.TxRandom.GetTxOTARandomPoint()
	if err != nil {
		return false, err
	}
	index, err := accessOTA.TxRandom.GetIndex()
	if err != nil {
		return false, err
	}
	pass := false
	otasecret := otakey.GetOTASecretKey()
	pubkey := accessOTA.PublicKey
	otapub := otakey.GetPublicSpend()

	rK := new(operation.Point).ScalarMult(txOTARandomPoint, otasecret)
	hashed := operation.HashToScalar(
		append(rK.ToBytesS(), common.Uint32ToBytes(index)...),
	)
	HnG := new(operation.Point).ScalarMultBase(hashed)
	KCheck := new(operation.Point).Sub(&pubkey, HnG)
	pass = operation.IsPointEqual(KCheck, otapub)

	if pass {
		return true, nil
	}
	return false, nil
}

func processWaitingContributions(list *map[string]*rawdbv2.Pdexv3Contribution) (map[string]shared.ContributionData, map[string]*privacy.OTAReceiver, error) {
	contrbList := make(map[string]shared.ContributionData)
	accessOTAList := make(map[string]*privacy.OTAReceiver)
	if list != nil {
		for pairHash, v := range *list {
			txhash := v.TxReqID().String()
			tx, err := database.DBGetTxByHash([]string{txhash})
			if err != nil {
				return nil, nil, err
			}
			if len(tx) == 0 {
				return nil, nil, errors.New("len(tx) == 0")
			}
			contrbData := shared.ContributionData{
				RequestTxs:       []string{txhash},
				PairHash:         pairHash,
				PoolID:           v.PoolPairID(),
				ContributeAmount: []string{fmt.Sprintf("%v", v.Amount())},
				ContributeTokens: []string{v.TokenID().String()},
				RequestTime:      tx[0].Locktime,
			}
			if len(v.OtaReceivers()) > 0 {
				if accessOTAStr, ok := v.OtaReceivers()[common.PdexAccessCoinID]; ok {
					access := &privacy.OTAReceiver{}
					err := access.FromString(accessOTAStr)
					if err != nil {
						return nil, nil, err
					}
					contrbData.AccessIDs = append(contrbData.AccessIDs, base64.StdEncoding.EncodeToString(access.PublicKey.ToBytesS()))
					accessOTAList[txhash] = access
				}
			}
			contrbList[txhash] = contrbData
		}
	}
	return contrbList, accessOTAList, nil
}

func getNextOTAWaitingContribution(otakey string) ([]shared.ContributionData, error) {
	var result []shared.ContributionData
	wl, err := wallet.Base58CheckDeserialize(otakey)
	if err != nil {
		return nil, err
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		return nil, errors.New("invalid otakey")
	}
	txList := []string{}
	preloadPdexStateV3.WaitingContributions.RLock()
	if list, ok := preloadPdexStateV3.WaitingContributions.cacheOTAList[otakey]; ok {
		for _, v := range list {
			result = append(result, preloadPdexStateV3.WaitingContributions.List[v])
		}
		preloadPdexStateV3.WaitingContributions.RUnlock()
		return result, nil
	} else {
		for txhash, v := range preloadPdexStateV3.WaitingContributions.AccessOTAs {
			pass, err := checkAccessOTABelongToKey(v, wl.KeySet.OTAKey)
			if err != nil {
				return nil, err
			}
			if pass {
				txList = append(txList, txhash)
				result = append(result, preloadPdexStateV3.WaitingContributions.List[txhash])
			}
		}
	}
	preloadPdexStateV3.WaitingContributions.RUnlock()
	preloadPdexStateV3.WaitingContributions.cacheOTAList[otakey] = txList
	return result, nil
}

func getRefundContribution(otakey string, filterTxs []string) ([]shared.ContributionData, error) {
	var result []shared.ContributionData
	wl, err := wallet.Base58CheckDeserialize(otakey)
	if err != nil {
		return nil, err
	}
	if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
		return nil, errors.New("invalid otakey")
	}
	pubkey := wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()
	pubkeyStr := base58.EncodeCheck(pubkey)
	txList, err := database.DBGetTxByMetaAndOTA(pubkeyStr, metadataCommon.Pdexv3AddLiquidityResponseMeta, 100000, 0)
	if err != nil {
		return nil, err
	}
	filterTxsMap := make(map[string]struct{})
	for _, v := range filterTxs {
		filterTxsMap[v] = struct{}{}
	}

	txToGet := []string{}
	for _, v := range txList {
		if _, ok := filterTxsMap[v.TxHash]; !ok {
			txToGet = append(txToGet, v.TxHash)
		}
	}
	list, err := database.DBGetPDEV3ContributeByRespondTx(txToGet)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if v.NFTID == "" {
			result = append(result, v)
		}
	}
	return result, nil
}
