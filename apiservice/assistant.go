package apiservice

import (
	"log"
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
			Price:         float64(v.Token1Amount) / float64(v.Token2Amount),
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
		Rate   float64
		MaxAMP int
	}
	token1 := c.Query("token1")
	token2 := c.Query("token2")
	amount1, _ := strconv.Atoi(c.Query("amount1"))
	amount2, _ := strconv.Atoi(c.Query("amount2"))
	amp, _ := strconv.Atoi(c.Query("amp"))

	userRate := float64(amount1) / float64(amount2)
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

				result.Rate = float64(tk1Price.Price) / float64(tk2Price.Price)
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

func getRate(tokenID1, tokenID2 string) (float64, error) {
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
		1000)

	spew.Dump("chosenPath", chosenPath)
	log.Printf("getRate %d\n", receive)
	return float64(1000) / float64(receive), nil
}
