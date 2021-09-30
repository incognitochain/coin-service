package apiservice

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/pdexv3/analyticsquery"
)

func APIGetTop10(c *gin.Context) {
	list, err := database.DBGetTop10PairHighestCap()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	//TODO cache default pool
	defaultPools, err := database.DBGetDefaultPool()
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
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

func APICheckRate(c *gin.Context) {
	var result struct {
		Rate uint64
		AMP  int
	}
	token1 := c.Query("token1")
	token2 := c.Query("token2")
	amount1, _ := strconv.Atoi(c.Query("amount1"))
	amount2, _ := strconv.Atoi(c.Query("amount2"))
	amp, _ := strconv.Atoi(c.Query("amp"))

	userRate := amount1 / amount2
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
			result.Rate = tk1Price.Price / tk2Price.Price
			result.AMP = amp
		} else {
			result.Rate = uint64(userRate)
			result.AMP = amp
		}
	} else {
		result.Rate = uint64(userRate)
		result.AMP = amp
	}
	respond := APIRespond{
		Result: result,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
