package assistant

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	pdexv3Meta "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

func getExternalPrice(tokenSymbol string) (float64, error) {
	var price struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
	var jsonErr struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	retryTimes := 0
retry:
	retryTimes++
	if retryTimes > 2 {
		return 0, nil
	}
	tk := strings.ToUpper(tokenSymbol)
	switch tk {
	case "USDT", "USDC":
		return 1, nil
	}
	resp, err := http.Get(binancePriceURL + tokenSymbol + "USDT")
	if err != nil {
		log.Println(err)
		goto retry
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(body, &price)
	if err != nil {
		err = json.Unmarshal(body, &jsonErr)
		if err != nil {
			log.Println(err)
			goto retry
		}
	}
	resp.Body.Close()
	value, err := strconv.ParseFloat(price.Price, 32)
	if err != nil {
		log.Println("getExternalPrice", tokenSymbol, err)
		return 0, nil
	}
	return value, nil
}

func getBridgeTokenExternalPrice() ([]shared.TokenPrice, error) {
	var result []shared.TokenPrice
	bridgeTokens, err := database.DBGetBridgeTokens()
	if err != nil {
		return nil, err
	}
	for _, v := range bridgeTokens {
		d, err := database.DBGetExtraTokenInfo(v.TokenID)
		if err != nil {
			return nil, err
		}
		price := float64(0)
		tkname := v.Name
		tkSymbol := v.Symbol
		if d != nil {
			price, err = getExternalPrice(d.Symbol)
			if err != nil {
				return nil, err
			}
			tkname = d.Name
			tkSymbol = d.Symbol
		} else {
			price, err = getExternalPrice(v.Symbol)
			if err != nil {
				return nil, err
			}
		}
		if price == 0 {
			fmt.Printf("price for token %v is 0\n", v.TokenID)
			continue
		}
		tokenPrice := shared.TokenPrice{
			TokenID:     v.TokenID,
			Price:       fmt.Sprintf("%g", price),
			TokenName:   tkname,
			TokenSymbol: tkSymbol,
			Time:        time.Now().Unix(),
		}
		result = append(result, tokenPrice)
	}
	return result, nil
}

func getExternalTokenMarketCap() ([]shared.TokenMarketCap, error) {
	var result []shared.TokenMarketCap
	var coingeckoMK []struct {
		Symbol      string  `json:"symbol"`
		Cap         uint64  `json:"market_cap"`
		Rank        int     `json:"market_cap_rank"`
		PriceChange float64 `json:"price_change_percentage_24h"`
	}
	currentPage := 1
	maxPage := 4
	for {
		if currentPage > maxPage {
			break
		}
	retry:
		resp, err := http.Get(coingeckoMkCapURL + strconv.Itoa(currentPage))
		if err != nil {
			log.Println(err)
			goto retry
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		err = json.Unmarshal(body, &coingeckoMK)
		if err != nil {
			log.Println(err)
			return nil, nil
		}
		resp.Body.Close()

		for _, v := range coingeckoMK {
			mkCap := shared.TokenMarketCap{
				TokenSymbol: strings.ToUpper(v.Symbol),
				Value:       v.Cap,
				Rank:        v.Rank,
				PriceChange: fmt.Sprintf("%g", v.PriceChange),
			}
			result = append(result, mkCap)
		}
		currentPage++
	}

	return result, nil
}

func getPairRanking() ([]shared.PairRanking, error) {
	//get default pools for now
	var result []shared.PairRanking
	var defaultPools map[string]struct{}
	if err := cacheGet(defaultPoolsKey, defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool()
		if err != nil {
			return nil, err
		}
		err = cacheStore(defaultPoolsKey, defaultPools)
		if err != nil {
			return nil, err
		}
	}
	if len(defaultPools) == 0 {
		return nil, nil
	}
	for v, _ := range defaultPools {
		pools, err := database.DBGetPoolPairsByPoolID([]string{v})
		if err != nil {
			return nil, err
		}
		if len(pools) == 0 {
			continue
		}
		pairRank := shared.PairRanking{
			LeadPool: v,
			PairID:   pools[0].PairID,
		}
		result = append(result, pairRank)
	}
	return result, nil
}

func getInternalTokenPrice() ([]shared.TokenInfoData, error) {
	var result []shared.TokenInfoData

	baseToken, err := database.DBGetBasePriceToken()
	if err != nil {
		return nil, err
	}
	if baseToken == "" {
		return nil, nil
	}
	tokenList, err := database.DBGetTokenInfo()
	if err != nil {
		return nil, err
	}

	for _, v := range tokenList {
		rate, err := getRate(baseToken, v.TokenID, 1, 1)
		if err != nil {
			log.Println("getInternalTokenPrice", err)
			continue
		}
		dcrate, err := getPdecimalRate(v.TokenID, baseToken)
		if err != nil {
			log.Println("getPdecimalRate", err)
			continue
		}

		rate = rate * dcrate
		v.CurrentPrice = fmt.Sprintf("%g", rate)
		if time.Since(v.UpdatedAt) >= 24*time.Hour {
			v.PastPrice = v.CurrentPrice
			v.UpdatedAt = time.Now().UTC()
		}
		result = append(result, v)
	}
	return result, nil
}

func getRate(tokenID1, tokenID2 string, token1Amount, token2Amount uint64) (float64, error) {
	data, err := database.DBGetPDEState(2)
	if err != nil {
		return 0, err
	}
	pdeState := jsonresult.Pdexv3State{}
	err = json.UnmarshalFromString(data, &pdeState)
	if err != nil {
		return 0, err
	}
	poolPairStates := *pdeState.PoolPairs
	var pools []*shared.Pdexv3PoolPairWithId
	for poolId, element := range poolPairStates {

		var poolPair rawdbv2.Pdexv3PoolPair
		var poolPairWithId shared.Pdexv3PoolPairWithId

		poolPair = element.State()
		poolPairWithId = shared.Pdexv3PoolPairWithId{
			poolPair,
			shared.Pdexv3PoolPairChild{
				PoolID: poolId},
		}

		pools = append(pools, &poolPairWithId)
	}
	a := uint64(1)
	a1 := uint64(0)
	b := uint64(1)
	b1 := uint64(0)
retry:
	_, receive := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a)

	if receive == 0 {
		a *= 10
		if a < 1e18 {
			goto retry
		}
		return 0, nil
	} else {
		if receive > a1*10 {
			a *= 10
			a1 = receive
			goto retry
		} else {
			if receive < a1*10 {
				a /= 10
				receive = a1
				fmt.Println("receive", a, receive)
			}
		}
	}

retry2:
	_, receive2 := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID2,
		tokenID1,
		b)

	if receive2 == 0 {
		b *= 10
		if b < 1e18 {
			goto retry2
		}
		return 0, nil
	} else {
		if receive2 > b1*10 {
			b *= 10
			b1 = receive2
			goto retry2
		} else {
			if receive2 < b1*10 {
				b /= 10
				receive2 = b1
				fmt.Println("receive2", b, receive2)
			}
		}
	}
	return (float64(a)/float64(receive) + (1 / (float64(b) / float64(receive2)))) / 2, nil
}

func getPdecimalRate(tokenID1, tokenID2 string) (float64, error) {
	tk1Decimal := 1
	tk2Decimal := 1
	tk1, err := database.DBGetExtraTokenInfo(tokenID1)
	if err != nil {
		log.Println(err)
	}
	tk2, err := database.DBGetExtraTokenInfo(tokenID2)
	if err != nil {
		log.Println(err)
	}
	if tk1 != nil {
		tk1Decimal = int(tk1.PDecimals)
	}
	if tk2 != nil {
		tk2Decimal = int(tk2.PDecimals)
	}
	result := math.Pow10(tk1Decimal) / math.Pow10(tk2Decimal)
	fmt.Println("getPdecimalRate", result)
	return result, nil
}
