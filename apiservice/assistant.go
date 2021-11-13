package apiservice

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

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
	defaultPoolsKey  string = "dfpool"
	tokenPriorityKey string = "tkpriority"
	tokenInfoKey     string = "tokenInfo"
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
		tk1Amount, _ := strconv.ParseUint(v.Token1Amount, 10, 64)
		tk2Amount, _ := strconv.ParseUint(v.Token2Amount, 10, 64)
		if tk1Amount == 0 || tk2Amount == 0 {
			continue
		}
		dcrate, err := getPdecimalRate(v.TokenID1, v.TokenID2)
		if err != nil {
			c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
			return
		}
		tk1VA, _ := strconv.ParseUint(v.Virtual1Amount, 10, 64)
		tk2VA, _ := strconv.ParseUint(v.Virtual2Amount, 10, 64)
		totalShare, _ := strconv.ParseUint(v.TotalShare, 10, 64)
		data := PdexV3PoolDetail{
			PoolID:         v.PoolID,
			Token1ID:       v.TokenID1,
			Token2ID:       v.TokenID2,
			Token1Value:    tk1Amount,
			Token2Value:    tk2Amount,
			Virtual1Value:  tk1VA,
			Virtual2Value:  tk2VA,
			PriceChange24h: 0,
			Volume:         0,
			AMP:            v.AMP,
			Price:          (float64(tk2Amount) / float64(tk1Amount)) * dcrate,
			TotalShare:     totalShare,
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
			newAmp := AMP_CLASS4
			tk1Price, err := strconv.ParseFloat(tk1Price.Price, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}
			tk2Price, err := strconv.ParseFloat(tk2Price.Price, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
				return
			}

			if len(mkcaps) == 2 {
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
			}
			rate := float64(tk1Price) / float64(tk2Price)
			result.Rate = fmt.Sprintf("%g", rate)
			result.MaxAMP = newAmp
			respond := APIRespond{
				Result: result,
				Error:  nil,
			}
			c.JSON(http.StatusOK, respond)
			return
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
		if a < token1Amount && receive > a1*10 {
			a *= 10
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
		if b < token2Amount && receive2 > b1*10 {
			b *= 10
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

func APIGetPdecimal(c *gin.Context) {
	data := []byte{}
	err := cacheGet("pdecimal", &data)
	if err != nil {
		log.Println(err)
	} else {
		respond := APIRespond{
			Result: data,
			Error:  nil,
		}
		c.JSON(http.StatusOK, respond)
		return
	}
	resp, err := http.Get(shared.ServiceCfg.ExternalDecimals)
	if err != nil {
		errStr := err.Error()
		respond := APIRespond{
			Result: nil,
			Error:  &errStr,
		}
		c.JSON(http.StatusOK, respond)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, buildGinErrorRespond(err))
		return
	}
	go cacheStoreCustom("pdecimal", body, 5*time.Minute)
	respond := APIRespond{
		Result: body,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
