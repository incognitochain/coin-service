package apiservice

import (
	"net/http"

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
		// data.Volume
		// data.PriceChange24h
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
	token1 := c.Query("token1")
	token2 := c.Query("token2")
	amount1 := c.Query("amount1")
	amount2 := c.Query("amount2")
	amp := c.Query("amp")

	_ = token1
	_ = token2
	_ = amount1
	_ = amount2
	_ = amp
}
