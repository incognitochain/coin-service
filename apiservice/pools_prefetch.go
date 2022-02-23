package apiservice

import (
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/shared"
)

var allPoolList []PdexV3PoolDetail
var verifyPoolList []PdexV3PoolDetail
var poolListLock sync.RWMutex

func getPoolList(isAll bool) []PdexV3PoolDetail {
	poolListLock.RLock()
	defer poolListLock.RUnlock()
	newList := []PdexV3PoolDetail{}
	if isAll {
		newList = append(newList, allPoolList...)
	} else {
		newList = append(newList, verifyPoolList...)
	}
	return newList
}

func poolListWatcher() {
	for {
		go retrievePoolList(true)
		go retrievePoolList(false)
		time.Sleep(15 * time.Second)
	}
}

func retrievePoolList(verify bool) {
	list, err := database.DBGetPoolPairsByPairID("all")
	if err != nil {
		log.Println(err)
		return
	}
	defaultPools, err := database.DBGetDefaultPool(true)
	if err != nil {
		log.Println(err)
		return
	}
	priorityTokens, err := database.DBGetTokenPriority()
	if err != nil {
		log.Println(err)
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
			isVerify := false
			if _, found := defaultPools[d.PoolID]; found {
				isVerify = true
			}
			if verify && !isVerify {
				return
			}
			if verify && isVerify {
				return
			}

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
				IsVerify:       isVerify,
			}

			poolLiquidityChanges, err := analyticsquery.APIGetPDexV3PairRateChangesAndVolume24h([]string{d.PoolID})
			if err != nil {
				log.Println(err)
				return
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
		}(v)
	}
	wg.Wait()
	close(resultCh)
	for v := range resultCh {
		result = append(result, v)
	}
	poolListLock.Lock()
	defer poolListLock.Unlock()
	if !verify {
		allPoolList = result
	} else {
		verifyPoolList = result
	}
}
