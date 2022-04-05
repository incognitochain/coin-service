package apiservice

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
)

var estokenList []TokenInfo
var alltokenList []TokenInfo
var marketTokenList []TokenInfo
var tokenMap map[string]int
var tokenListLock sync.RWMutex

func getTokenList(isAll bool) []TokenInfo {
	tokenListLock.RLock()
	defer tokenListLock.RUnlock()
	newList := []TokenInfo{}
	if isAll {
		newList = append(newList, alltokenList...)
	} else {
		newList = append(newList, estokenList...)
	}
	return newList
}

func getMarketTokenList() []TokenInfo {
	tokenListLock.RLock()
	defer tokenListLock.RUnlock()

	newList := []TokenInfo{}
	newList = append(newList, marketTokenList...)
	return newList

}

func getCustomTokenList(tokenList []string) []TokenInfo {
	tokenListLock.RLock()
	defer tokenListLock.RUnlock()
	newList := []TokenInfo{}
	for _, v := range tokenList {
		tkIdx, ok := tokenMap[v]
		if ok {
			newList = append(newList, alltokenList[tkIdx])
		}
	}

	return newList
}

func tokenListWatcher() {
	tokenMap = make(map[string]int)
	for {
		retrieveTokenList()
		time.Sleep(15 * time.Second)
	}
}

func retrieveTokenList() {
	startTime := time.Now()
	var wg sync.WaitGroup
	wg.Add(4)

	var extraTokenInfo []shared.ExtraTokenInfo
	var customTokenInfo []shared.CustomTokenInfo
	var defaultPools map[string]struct{}
	var priorityTokens []string
	var datalist []TokenInfo

	go func() {
		defer wg.Done()
		var err error
		extraTokenInfo, err = database.DBGetAllExtraTokenInfo()
		if err != nil {
			log.Println(err)
			return
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		customTokenInfo, err = database.DBGetAllCustomTokenInfo()
		if err != nil {
			log.Println(err)
			return
		}
	}()

	go func() {
		var err error
		defer wg.Done()
		defaultPools, err = database.DBGetDefaultPool(true)
		if err != nil {
			log.Println(err)
			return
		}
	}()
	go func() {
		var err error
		defer wg.Done()
		priorityTokens, err = database.DBGetTokenPriority()
		if err != nil {
			log.Println(err)
			return
		}
	}()
	wg.Wait()
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
	var err error
	var tokenList []shared.TokenInfoData
	tokenList, err = database.DBGetAllTokenInfo()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("APIGetTokenList1", time.Since(startTime))
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
			if etki.Image != "" {
				data.Image = etki.Image
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
			data.ExternalPriceUSD = etki.PriceUsd
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
			err = json.UnmarshalFromString(etki.ListUnifiedToken, &data.ListUnifiedToken)
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

	fmt.Println("APIGetTokenList2", time.Since(startTime))
	for _, tkInfo := range extraTokenInfo {
		if _, ok := chainTkListMap[tkInfo.TokenID]; !ok {
			tkdata := TokenInfo{
				TokenID:            tkInfo.TokenID,
				Name:               tkInfo.Name,
				Symbol:             tkInfo.Symbol,
				PSymbol:            tkInfo.PSymbol,
				PDecimals:          int(tkInfo.PDecimals),
				Decimals:           tkInfo.Decimals,
				ContractID:         tkInfo.ContractID,
				Status:             tkInfo.Status,
				Type:               tkInfo.Type,
				CurrencyType:       tkInfo.CurrencyType,
				Default:            tkInfo.Default,
				Verified:           tkInfo.Verified,
				UserID:             tkInfo.UserID,
				ExternalPriceUSD:   tkInfo.PriceUsd,
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
			err = json.UnmarshalFromString(tkInfo.ListUnifiedToken, &tkdata.ListUnifiedToken)
			if err != nil {
				panic(err)
			}
			datalist = append(datalist, tkdata)
		}
	}

	tokenListLock.Lock()
	defer tokenListLock.Unlock()
	alltokenList = datalist
	tokenMap = make(map[string]int)
	for idx, v := range alltokenList {
		tokenMap[v.TokenID] = idx
	}

	estokenList = []TokenInfo{}
	for _, v := range datalist {
		if v.Verified {
			estokenList = append(estokenList, v)
		} else {
			if _, ok := extraTokenInfoMap[v.TokenID]; ok {
				estokenList = append(estokenList, v)
			}
		}
	}

	tokenMarketMap := make(map[string]struct{})
	marketTokenList = []TokenInfo{}
	for poolID := range defaultPools {
		tks := strings.Split(poolID, "-")
		tokenMarketMap[tks[0]] = struct{}{}
		tokenMarketMap[tks[1]] = struct{}{}
	}
	for tokenID := range tokenMarketMap {
		marketTokenList = append(marketTokenList, datalist[tokenMap[tokenID]])
	}
}
