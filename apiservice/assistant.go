package apiservice

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	pdexv3Meta "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

const (
	defaultPoolsKey string = "dfpool"
)

func APIGetTop10(c *gin.Context) {
	list, err := database.DBGetTop10PairHighestCap()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var defaultPools map[string]struct{}
	if err := cacheGet(defaultPoolsKey, defaultPools); err != nil {
		defaultPools, err = database.DBGetDefaultPool()
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

	poolIDs := []string{}
	for _, v := range list {
		poolIDs = append(poolIDs, v.LeadPool)
	}
	poolLiquidityChanges, err := analyticsquery.APIGetPDexV3PairRateChangesAndVolume24h(poolIDs)

	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	poolList, err := database.DBGetPoolPairsByPoolID(poolIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	var result []PdexV3PoolDetail
	for _, v := range poolList {
		data := PdexV3PoolDetail{
			PoolID:        v.PoolID,
			Token1ID:      v.TokenID1,
			Token2ID:      v.TokenID2,
			Token1Value:   v.Token1Amount,
			Token2Value:   v.Token2Amount,
			Virtual1Value: v.Virtual1Amount,
			Virtual2Value: v.Virtual2Amount,
			AMP:           v.AMP,
			Price:         float64(v.Token1Amount) / float64(v.Token2Amount),
			TotalShare:    v.TotalShare,
		}

		if poolChange, found := poolLiquidityChanges[v.PoolID]; found {
			data.PriceChange24h = poolChange.RateChangePercentage
			data.Volume = poolChange.TradingVolume24h
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

const (
	// PriceChange_CorrelationSimilar = float64(0.05)
	// PriceChange_CorrelationStrong  = float64(0.2)
	// PriceChange_CorrelationAverage = float64(0.5)
	// PriceChange_CorrelationWeak    = float64(1)

	// PriceChange_RangeStable = float64(1)
	// PriceChange_RangeLow    = float64(2)
	// PriceChange_RangeHigh   = float64(6)

	AMP_CLASS1 = 200
	AMP_CLASS2 = 10
	AMP_CLASS3 = 2
	AMP_CLASS4 = 1
)

func APICheckRate(c *gin.Context) {
	var result struct {
		Rate   string
		MaxAMP int
	}
	token1 := c.Query("token1")
	token2 := c.Query("token2")
	amount1, _ := strconv.Atoi(c.Query("amount1"))
	amount2, _ := strconv.Atoi(c.Query("amount2"))
	amp, _ := strconv.Atoi(c.Query("amp"))

	userRate := float64(amount2) / float64(amount1)
	tk1Price, err := database.DBGetTokenPrice(token1)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	dcrate, err := getPdecimalRate(token1, token2)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	if tk1Price != nil {
		tk2Price, err := database.DBGetTokenPrice(token2)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if tk2Price != nil {
			tokenSymbols := []string{tk1Price.TokenSymbol, tk2Price.TokenSymbol}
			mkcaps, err := database.DBGetTokenMkcap(tokenSymbols)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if len(mkcaps) == 2 {
				newAmp := AMP_CLASS4
				isTk1Stable := false
				isTk2stable := false
				stableCoinList, err := database.DBGetStableCoinID()
				if err != nil {
					c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
					return
				}
				for _, v := range stableCoinList {
					if v == token1 {
						isTk1Stable = true
					}
					if v == token2 {
						isTk2stable = true
					}
				}
				if isTk1Stable && isTk2stable {
					if tk1Price == tk2Price {
						newAmp = AMP_CLASS1
					} else {
						newAmp = AMP_CLASS2
					}
				} else {
					if tk1Price == tk2Price {
						newAmp = AMP_CLASS1
					} else {
						if (mkcaps[0].Rank <= 20) && (mkcaps[1].Rank <= 20) {
							newAmp = AMP_CLASS3
						}
					}
				}
				rate := float64(tk2Price.Price) / float64(tk1Price.Price)
				fmt.Printf("result.Rate2 %v %v \n", float64(tk1Price.Price), float64(tk2Price.Price))
				fmt.Printf("result.Rate2 %v \n", rate)
				result.Rate = fmt.Sprintf("%g", rate*dcrate)
				result.MaxAMP = newAmp
				respond := APIRespond{
					Result: result,
					Error:  nil,
				}
				c.JSON(http.StatusOK, respond)
				return
			}
		} else {
			rate, err := getRate(token1, token2, uint64(amount1), uint64(amount2))
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			if rate != 0 {
				userRate = 1 / rate
			}
		}
	} else {
		rate, err := getRate(token1, token2, uint64(amount1), uint64(amount2))
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		if rate != 0 {
			userRate = 1 / rate
		}
	}
	fmt.Printf("result.Rate %v \n", userRate*dcrate)
	result.Rate = fmt.Sprintf("%g", userRate*dcrate)
	result.MaxAMP = amp
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
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

	// pools, poolPairStates, err := pathfinder.GetPdexv3PoolDataFromRawRPCResult(pdexv3StateRPCResponse.Result.Poolpairs)
	// if err != nil {
	// 	return 0, err
	// }

	// _, err = feeestimator.GetPdexv3PoolDataFromRawRPCResult(pdexv3StateRPCResponse.Result.Params, pdexv3StateRPCResponse.Result.Poolpairs)
	// if err != nil {
	// 	return 0, err
	// }
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
		if a < token1Amount && receive > a1 {
			a1 = receive
			goto retry
		} else {
			if receive < a1 {
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
		if b < token2Amount && receive2 > b1 {
			b1 = receive2
			goto retry2
		} else {
			if receive2 < b1 {
				b /= 10
				receive2 = b1
				fmt.Println("receive2", b, receive2)
			}
		}
	}

	log.Printf("getRate %v %d\n", a, receive)
	log.Printf("getRate %v %d\n", b, receive2)
	return (float64(a)/float64(receive) + (1 / (float64(b) / float64(receive2)))) / 2, nil
}

func getPdecimalRate(tokenID1, tokenID2 string) (float64, error) {
	tk1Decimal := 1
	tk2Decimal := 1
	tk1, err := database.DBGetTokenDecimal(tokenID1)
	if err != nil {
		log.Println(err)
	}
	tk2, err := database.DBGetTokenDecimal(tokenID2)
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
