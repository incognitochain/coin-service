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
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
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
		if d == nil {
			continue
		}
		if !d.Verified {
			continue
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
		}
		//  else {
		// 	price, err = getExternalPrice(v.Symbol)
		// 	if err != nil {
		// 		return nil, err
		// 	}
		// }
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
	if err := cacheGet(defaultPoolsKey, &defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool(true)
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
	stableCoins, err := database.DBGetStableCoinID()
	if err != nil {
		return nil, err
	}
	stableCoins = append(stableCoins, baseToken)
	stableCoinStr := strings.Join(stableCoins, ",")
	defaultPools, err := database.DBGetDefaultPool(true)
	if err != nil {
		return nil, err
	}
	if baseToken == "" {
		return nil, nil
	}
	tokenList, err := database.DBGetAllTokenInfo()
	if err != nil {
		return nil, err
	}
	prvPrice := getPRVPrice(defaultPools, baseToken)
	data, err := database.DBGetPDEState(2)
	if err != nil {
		return nil, err
	}
	pdeState := jsonresult.Pdexv3State{}
	err = json.UnmarshalFromString(data, &pdeState)
	if err != nil {
		return nil, err
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
	for _, v := range tokenList {
		if v.TokenID == common.PRVCoinID.String() {
			v.CurrentPrice = strconv.FormatFloat(prvPrice, 'f', -1, 64)
		} else {
			if v.TokenID != baseToken {
				var rate float64
				poolAmountTk := uint64(0)
				poolAmountTkBase := uint64(0)

				filterPools := make(map[string]uint64)
				for poolID, _ := range defaultPools {
					if v.TokenID != common.PRVCoinID.String() {
						tks := strings.Split(poolID, "-")
						if strings.Contains(poolID, v.TokenID) && (strings.Contains(stableCoinStr, tks[0]) || strings.Contains(stableCoinStr, tks[1]) || strings.Contains(poolID, common.PRVCoinID.String())) {
							pool := getPool(poolID)
							if pool == nil {
								continue
							}
							tokenAmount := uint64(0)
							if tks[0] == v.TokenID {
								tokenAmount, _ = strconv.ParseUint(pool.Token1Amount, 10, 64)
							} else {
								tokenAmount, _ = strconv.ParseUint(pool.Token2Amount, 10, 64)
							}
							filterPools[poolID] = tokenAmount
						}
					}
				}

				chosenPoolID := ""
				tempPoolAmount := uint64(0)
				for poolID, tokenAmount := range filterPools {
					if tokenAmount > tempPoolAmount {
						chosenPoolID = poolID
						tempPoolAmount = tokenAmount
					}
				}

				if chosenPoolID == "" {
					continue
				}

				tks := strings.Split(chosenPoolID, "-")
				if strings.Contains(stableCoinStr, tks[0]) || strings.Contains(stableCoinStr, tks[1]) {
					pool := getPool(chosenPoolID)
					if pool == nil {
						continue
					}
					if tks[0] == v.TokenID {
						dcrate, _, _, err := getPdecimalRate(v.TokenID, tks[1])
						if err != nil {
							log.Println("getPdecimalRate", err)
							continue
						}
						tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
						tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
						if tk1Amount == 0 || tk2Amount == 0 {
							continue
						}
						if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
							continue
						}
						poolAmountTk = tk1Amount
						poolAmountTkBase = tk2Amount
						tk1VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
						tk2VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
						// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
						rate = calcRateSimple(float64(tk1VA), float64(tk2VA)) * dcrate
					} else {
						dcrate, _, _, err := getPdecimalRate(v.TokenID, tks[0])
						if err != nil {
							log.Println("getPdecimalRate", err)
							continue
						}
						tk1Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
						tk2Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
						if tk1Amount == 0 || tk2Amount == 0 {
							continue
						}
						if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
							continue
						}
						poolAmountTk = tk2Amount
						poolAmountTkBase = tk1Amount
						tk1VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
						tk2VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
						// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
						rate = calcRateSimple(float64(tk1VA), float64(tk2VA)) * dcrate
					}
					// if rate > 0 {
					// 	rate = rate * dcrate
					// }
				} else {
					dcrate, _, _, err := getPdecimalRate(v.TokenID, common.PRVCoinID.String())
					if err != nil {
						log.Println("getPdecimalRate", err)
						continue
					}
					pool := getPool(chosenPoolID)
					if pool == nil {
						continue
					}
					tks := strings.Split(chosenPoolID, "-")
					if tks[0] == common.PRVCoinID.String() {
						tk1Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
						tk2Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
						if tk1Amount == 0 || tk2Amount == 0 {
							continue
						}
						if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
							continue
						}
						poolAmountTk = tk2Amount
						poolAmountTkBase = tk1Amount
						tk1VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
						tk2VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
						// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
						rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
					} else {
						tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
						tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
						if tk1Amount == 0 || tk2Amount == 0 {
							continue
						}
						if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
							continue
						}
						poolAmountTk = tk1Amount
						poolAmountTkBase = tk2Amount
						tk1VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
						tk2VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
						// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
						rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
					}
					rate = rate * dcrate * prvPrice
				}

				// for poolID, _ := range defaultPools {
				// 	if strings.Contains(poolID, v.TokenID) && strings.Contains(poolID, baseToken) {
				// 		pool := getPool(poolID)
				// 		if pool == nil {
				// 			continue
				// 		}
				// 		tks := strings.Split(poolID, "-")
				// 		if tks[0] == baseToken {
				// 			tk1Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				// 			tk2Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				// 			if tk1Amount == 0 || tk2Amount == 0 {
				// 				continue
				// 			}
				// 			if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
				// 				continue
				// 			}
				// 			poolAmountTk = tk2Amount
				// 			poolAmountTkBase = tk1Amount
				// 			tk1VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				// 			tk2VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				// 			// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
				// 			rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
				// 		} else {
				// 			tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				// 			tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				// 			if tk1Amount == 0 || tk2Amount == 0 {
				// 				continue
				// 			}
				// 			if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
				// 				continue
				// 			}
				// 			poolAmountTk = tk1Amount
				// 			poolAmountTkBase = tk2Amount
				// 			tk1VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				// 			tk2VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				// 			// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
				// 			rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
				// 		}
				// 		if rate > 0 {
				// 			rate = rate * dcrate
				// 		}
				// 	}
				// }
				// if rate == 0 {
				// 	tokenPools := []string{}
				// 	for p, _ := range defaultPools {
				// 		if strings.Contains(p, v.TokenID) && strings.Contains(p, common.PRVCoinID.String()) {
				// 			tokenPools = append(tokenPools, p)
				// 		}
				// 	}
				// 	dcrate, _, _, err := getPdecimalRate(v.TokenID, common.PRVCoinID.String())
				// 	if err != nil {
				// 		log.Println("getPdecimalRate", err)
				// 		continue
				// 	}

				// 	for _, poolID := range tokenPools {
				// 		pool := getPool(poolID)
				// 		if pool == nil {
				// 			continue
				// 		}
				// 		tks := strings.Split(poolID, "-")
				// 		if tks[0] == common.PRVCoinID.String() {
				// 			tk1Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				// 			tk2Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				// 			if tk1Amount == 0 || tk2Amount == 0 {
				// 				continue
				// 			}
				// 			if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
				// 				continue
				// 			}
				// 			poolAmountTk = tk2Amount
				// 			poolAmountTkBase = tk1Amount
				// 			tk1VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				// 			tk2VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				// 			// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
				// 			rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
				// 		} else {
				// 			tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				// 			tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				// 			if tk1Amount == 0 || tk2Amount == 0 {
				// 				continue
				// 			}
				// 			if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
				// 				continue
				// 			}
				// 			poolAmountTk = tk1Amount
				// 			poolAmountTkBase = tk2Amount
				// 			tk1VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				// 			tk2VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				// 			// rate = calcAMPRate(float64(tk1VA), float64(tk2VA), float64(tk1Amount)/100)
				// 			rate = calcRateSimple(float64(tk1VA), float64(tk2VA))
				// 		}
				// 		if rate > 0 {
				// 			rate = rate * dcrate * prvPrice
				// 			break
				// 		}
				// 	}
				// }
				v.CurrentPrice = strconv.FormatFloat(rate, 'f', -1, 64)
			} else {
				v.CurrentPrice = "1"
			}
		}

		if time.Since(v.UpdatedAt) >= 24*time.Hour {
			v.PastPrice = v.CurrentPrice
			v.UpdatedAt = time.Now().UTC()
		}
		currPrice, _ := strconv.ParseFloat(v.CurrentPrice, 64)
		pastPrice, _ := strconv.ParseFloat(v.PastPrice, 64)

		if pastPrice == 0 && currPrice != 0 {
			v.PastPrice = v.CurrentPrice
			v.UpdatedAt = time.Now().UTC()
		}
		result = append(result, v)
	}
	return result, nil
}

func getRateMinimum(tokenID1, tokenID2 string, minAmount uint64, pools []*shared.Pdexv3PoolPairWithId, poolPairStates map[string]*pdex.PoolPairState, feeRateBPS uint) (float64, uint64, uint64) {
	a := uint64(minAmount)
	a1 := uint64(0)
retry:
	_, receive := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a, feeRateBPS)

	if receive == 0 {
		a *= 10
		if a < 1e9 {
			goto retry
		}
		return 0, 0, 0
	} else {
		if receive > a1*10 {
			a *= 10
			a1 = receive
			goto retry
		} else {
			a /= 10
			receive = a1
		}
	}
	return float64(receive) / float64(a), receive, a
	// return 0
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
retry:
	_, receive := pathfinder.FindGoodTradePath(
		pdexv3Meta.MaxTradePathLength,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		a, pdeState.Params.DefaultFeeRateBPS)

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
	return float64(receive) / float64(a), nil
}

func getPdecimalRate(tokenID1, tokenID2 string) (float64, int, int, error) {
	tk1Decimal := 0
	tk2Decimal := 0
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
	return result, tk1Decimal, tk2Decimal, nil
}

func getPool(poolID string) *shared.PoolInfoData {
	datas, err := database.DBGetPoolPairsByPoolID([]string{poolID})
	if err != nil {
		fmt.Println("poolID cant get", poolID)
		return nil
	}
	if len(datas) > 0 {
		result := datas[0]
		return &result
	}
	fmt.Println("poolID amount is zero", poolID)
	return nil
}

// func calcAMPRate(virtA, virtB, sellAmount float64) float64 {
// 	var result float64
// 	k := virtA * virtB
// 	result = virtB - (k / (virtA + sellAmount))
// 	return result / sellAmount
// }

func calcRateSimple(virtA, virtB float64) float64 {
	return virtB / virtA
}

func getPRVPrice(defaultPools map[string]struct{}, baseToken string) float64 {
	var result float64
	poolAmountTk := uint64(0)
	poolAmountTkBase := uint64(0)
retry:
	dcrate, _, _, err := getPdecimalRate(common.PRVCoinID.String(), baseToken)
	if err != nil {
		log.Println("getPdecimalRate", err)
		goto retry
	}
	for poolID, _ := range defaultPools {
		if strings.Contains(poolID, common.PRVCoinID.String()) && strings.Contains(poolID, baseToken) {
			pool := getPool(poolID)
			if pool == nil {
				continue
			}
			tks := strings.Split(poolID, "-")
			if tks[0] == baseToken {
				tk1Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				tk2Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				if tk1Amount == 0 || tk2Amount == 0 {
					continue
				}
				if tk2Amount < poolAmountTk && tk1Amount < poolAmountTkBase {
					continue
				}
				poolAmountTk = tk2Amount
				poolAmountTkBase = tk1Amount
				tk1VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				tk2VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				result = calcRateSimple(float64(tk1VA), float64(tk2VA))
			} else {
				tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
				tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
				if tk1Amount == 0 || tk2Amount == 0 {
					continue
				}
				if tk1Amount < poolAmountTk && tk2Amount < poolAmountTkBase {
					continue
				}
				poolAmountTk = tk1Amount
				poolAmountTkBase = tk2Amount
				tk1VA, _ := strconv.ParseUint(pool.Virtual1Amount, 10, 64)
				tk2VA, _ := strconv.ParseUint(pool.Virtual2Amount, 10, 64)
				result = calcRateSimple(float64(tk1VA), float64(tk2VA))
			}
		}
	}
	return result * dcrate
}
