package apiservice

import (
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/shared"
)

var allPoolList []PdexV3PoolDetail
var verifyPoolList []PdexV3PoolDetail
var poolMap map[string]int
var poolListLock sync.RWMutex

func getPoolList(verifyOnly bool) []PdexV3PoolDetail {
	poolListLock.RLock()
	defer poolListLock.RUnlock()
	newList := []PdexV3PoolDetail{}
	if verifyOnly {
		newList = append(newList, verifyPoolList...)
	} else {
		newList = append(newList, allPoolList...)
	}
	return newList
}

func getPoolListByPairID(pairID string, verifyOnly bool) []PdexV3PoolDetail {
	poolListLock.RLock()
	defer poolListLock.RUnlock()
	newList := []PdexV3PoolDetail{}
	for _, v := range allPoolList {
		if strings.Contains(pairID, v.Token1ID) && strings.Contains(pairID, v.Token2ID) {
			if verifyOnly && !v.IsVerify {
				continue
			}
			newList = append(newList, v)
		}
	}
	return newList
}

func getCustomPoolList(poolList []string) []PdexV3PoolDetail {
	poolListLock.RLock()
	defer poolListLock.RUnlock()
	newList := []PdexV3PoolDetail{}
	for _, v := range poolList {
		pIdx, ok := poolMap[v]
		if ok {
			newList = append(newList, allPoolList[pIdx])
		}
	}
	return newList
}

func poolListWatcher() {
	poolMap = make(map[string]int)
	for {
		retrievePoolList()
		time.Sleep(15 * time.Second)
	}
}

func retrievePoolList() {
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
				willSwapToken:  willSwap,
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
	poolVolumeToCheck := []string{}
	mapPool := make(map[string]PdexV3PoolDetail)
	for v := range resultCh {
		mapPool[v.PoolID] = v
		poolVolumeToCheck = append(poolVolumeToCheck, v.PoolID)
	}

	poolLiquidityChanges, err := analyticsquery.APIGetPDexV3PairRateChangesAndVolume24h(poolVolumeToCheck)
	if err != nil {
		log.Println(err)
		return
	}

	for poolID, pool := range mapPool {
		if d, ok := poolLiquidityChanges[poolID]; ok {
			if pool.willSwapToken {
				pool.PriceChange24h = ((1 / (1 + d.RateChangePercentage/100)) - 1) * 100
				pool.Volume = d.TradingVolume24h
			} else {
				pool.PriceChange24h = d.RateChangePercentage
				pool.Volume = d.TradingVolume24h
			}
		}
		result = append(result, pool)
	}

	poolListLock.Lock()
	defer poolListLock.Unlock()
	poolMap = make(map[string]int)
	allPoolList = result
	verifyPoolList = []PdexV3PoolDetail{}
	for _, v := range result {
		if v.IsVerify {
			if tokenMap != nil {
				value := v.Token1Value * 2
				tokenListLock.RLock()
				tkIdx, ok := tokenMap[v.Token1ID]
				tokenListLock.RUnlock()
				if ok {
					v.TotalValueLockUSD = float64(value) * (alltokenList[tkIdx].PriceUsd) * math.Pow10(-alltokenList[tkIdx].PDecimals)
				}
			}
			verifyPoolList = append(verifyPoolList, v)
		}
	}
	for idx, v := range allPoolList {
		poolMap[v.PoolID] = idx
	}
}
