package assistant

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
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
		return 0, err
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
		price, err := getExternalPrice(v.Symbol)
		if err != nil {
			return nil, err
		}
		if price == 0 {
			fmt.Printf("price for token %v is 0\n", v.TokenID)
			continue
		}
		priceNano := price * float64(1e9)
		tokenPrice := shared.TokenPrice{
			TokenID:     v.TokenID,
			Price:       uint64(priceNano),
			TokenName:   v.Name,
			TokenSymbol: v.Symbol,
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
	for v, _ := range defaultPools {
		pools, err := database.DBGetPoolPairsByPoolID([]string{v})
		if err != nil {
			return nil, err
		}
		pairRank := shared.PairRanking{
			LeadPool: v,
			PairID:   pools[0].PairID,
		}
		result = append(result, pairRank)
	}
	return result, nil
}
