package apiservice

import (
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
	"github.com/incognitochain/coin-service/pdexv3/pathfinder"
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
			Price:         v.Token1Amount / v.Token2Amount,
			TotalShare:    v.TotalShare,
		}

		//TODO @yenle add pool volume and price change 24h
		// data.APY

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

func APIGetDefaultPool(c *gin.Context) {
}

const (
	PriceChange_CorrelationSimilar = float64(0.05)
	PriceChange_CorrelationStrong  = float64(0.2)
	PriceChange_CorrelationAverage = float64(0.5)
	PriceChange_CorrelationWeak    = float64(1)

	PriceChange_RangeStable = float64(1)
	PriceChange_RangeLow    = float64(2)
	PriceChange_RangeHigh   = float64(6)
)

func APICheckRate(c *gin.Context) {
	var result struct {
		Rate   uint64
		MaxAMP int
	}
	token1 := c.Query("token1")
	token2 := c.Query("token2")
	amount1, _ := strconv.Atoi(c.Query("amount1"))
	amount2, _ := strconv.Atoi(c.Query("amount2"))
	amp, _ := strconv.Atoi(c.Query("amp"))

	userRate := uint64(amount1) / uint64(amount2)
	tk1Price, err := database.DBGetTokenPrice(token1)
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
				newAmp := 1
				tk1PriceChange, _ := strconv.ParseFloat(mkcaps[0].PriceChange, 32)
				tk2PriceChange, _ := strconv.ParseFloat(mkcaps[1].PriceChange, 32)
				if (mkcaps[0].Rank <= 100) && (mkcaps[1].Rank <= 100) {
					if tk1PriceChange*tk2PriceChange > 0 {
						d := math.Abs(tk1PriceChange - tk2PriceChange)
						if d < PriceChange_CorrelationWeak {
							if d < PriceChange_CorrelationAverage {
								newAmp = 2
								if d < PriceChange_CorrelationStrong {
									newAmp = 20
									if d < PriceChange_CorrelationSimilar {
										d2 := math.Abs(float64(mkcaps[0].Rank - mkcaps[1].Rank))
										if d2 <= 10 {
											newAmp = 100
										} else {
											if d2 <= 20 {
												newAmp = 70
											} else {
												if d2 <= 30 {
													newAmp = 40
												} else {
													newAmp = 25
												}
											}
										}
									}
									// if tk1PriceChange >= PriceChange_RangeStable || tk2PriceChange >= PriceChange_RangeStable {
									// 	newAmp -= 5
									// }
									// if tk1PriceChange >= PriceChange_RangeLow || tk2PriceChange >= PriceChange_RangeLow {
									// 	newAmp -= 5
									// }
									// if tk1PriceChange >= PriceChange_RangeHigh || tk2PriceChange >= PriceChange_RangeHigh {
									// 	newAmp -= 5
									// }
								}
							}
						}
					}
				}

				result.Rate = tk1Price.Price / tk2Price.Price
				result.MaxAMP = newAmp
				respond := APIRespond{
					Result: result,
					Error:  nil,
				}
				c.JSON(http.StatusOK, respond)
				return
			}
		} else {
			rate, err := getRate(token1, token2)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			userRate = rate
		}
	} else {
		rate, err := getRate(token1, token2)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		userRate = rate
	}
	result.Rate = userRate
	result.MaxAMP = amp
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}

func getRate(tokenID1, tokenID2 string) (uint64, error) {

	pdexv3StateRPCResponse, err := pathfinder.GetPdexv3StateFromRPC()

	if err != nil {
		return 0, err
	}

	pools, poolPairStates, err := pathfinder.GetPdexv3PoolDataFromRawRPCResult(pdexv3StateRPCResponse.Result.Poolpairs)

	if err != nil {
		return 0, err
	}

	// _, err = feeestimator.GetPdexv3PoolDataFromRawRPCResult(pdexv3StateRPCResponse.Result.Params, pdexv3StateRPCResponse.Result.Poolpairs)
	// if err != nil {
	// 	return 0, err
	// }

	chosenPath, receive := pathfinder.FindGoodTradePath(
		4,
		pools,
		poolPairStates,
		tokenID1,
		tokenID2,
		1e9)

	spew.Dump("chosenPath", chosenPath)
	log.Printf("getRate %d\n", receive)
	return receive, nil
}
