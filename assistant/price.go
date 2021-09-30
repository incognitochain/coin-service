package assistant

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
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
	var binanceMK struct {
		Data []struct {
			CS uint64 `json:"cs"`
			C  string `json:"c"`
			Q  string `json:"q"`
			B  string `json:"b"`
		} `json:"data"`
	}
retry:
	resp, err := http.Get(binanceMkCapURL)
	if err != nil {
		log.Println(err)
		goto retry
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(body, &binanceMK)
	if err != nil {
		log.Println(err)
		return nil, nil
	}
	resp.Body.Close()

	for _, v := range binanceMK.Data {
		if v.Q == "USDT" {
			price, err := strconv.ParseFloat(v.C, 32)
			if err != nil {
				return nil, err
			}
			value := price * float64(v.CS)
			mkCap := shared.TokenMarketCap{
				TokenSymbol: v.B,
				Value:       uint64(value),
			}
			result = append(result, mkCap)
		}
	}
	return result, nil
}

func getPairRanking() ([]shared.PairRanking, error) {
	//get default pools for now
	//TODO
	var result []shared.PairRanking
	defaultPools, err := database.DBGetDefaultPool()
	if err != nil {
		return nil, err
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
