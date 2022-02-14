package assistant

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
)

func checkPoolQualify(extraTokenInfo []shared.ExtraTokenInfo, customToken []shared.CustomTokenInfo) (string, error) {
	var qualifyPools string
	verifiedTks := make(map[string]struct{})
	tokenDecimal := make(map[string]int)
	for _, v := range extraTokenInfo {
		if v.Verified {
			verifiedTks[v.TokenID] = struct{}{}
			tokenDecimal[v.TokenID] = int(v.PDecimals)
		}
	}
	tokenDecimal[common.PRVCoinID.String()] = 9
	for _, v := range customToken {
		if v.Verified {
			verifiedTks[v.TokenID] = struct{}{}
			if _, ok := tokenDecimal[v.TokenID]; !ok {
				tokenDecimal[v.TokenID] = 0
			}
		}
	}

	baseTk, err := database.DBGetBasePriceToken()
	if err != nil {
		return "", err
	}
	stableCoins, err := database.DBGetStableCoinID()
	if err != nil {
		return "", err
	}

	stableCoins = append(stableCoins, baseTk)
	pools, err := database.DBGetPoolPairsByPairID("all")
	if err != nil {
		return "", err
	}

	defaultPools, err := database.DBGetDefaultPool(false)
	if err != nil {
		return "", err
	}
	prvPrice := getPRVPrice(defaultPools, baseTk)
	if prvPrice == 0 {
		return "", nil
	}
	poolsLq := make(map[string]uint64)
	for _, pool := range pools {
		_, ok1 := verifiedTks[pool.TokenID1]
		_, ok2 := verifiedTks[pool.TokenID2]
		if ok1 && ok2 {
			q1 := false
			tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
			tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
			liquidity := uint64(0)
			if pool.TokenID1 == common.PRVCoinID.String() || pool.TokenID2 == common.PRVCoinID.String() {
				q1 = true
				if pool.TokenID1 == common.PRVCoinID.String() {
					liquidity = uint64(float64(tk1Amount) / math.Pow10(9) * prvPrice * 2)
				} else {
					liquidity = uint64(float64(tk2Amount) / math.Pow10(9) * prvPrice * 2)
				}
			} else {
				for _, v := range stableCoins {
					if pool.TokenID1 == v || pool.TokenID2 == v {
						q1 = true
						if pool.TokenID1 == v {
							liquidity = uint64(float64(tk1Amount) * 2 / math.Pow10(tokenDecimal[pool.TokenID1]))
						} else {
							liquidity = uint64(float64(tk2Amount) * 2 / math.Pow10(tokenDecimal[pool.TokenID1]))
						}
						break
					}
				}
			}
			if q1 && liquidity >= mininumQualifyLiquidity {
				matchPool := ""
				for poolID, lq := range poolsLq {
					if strings.Contains(poolID, pool.TokenID1) && strings.Contains(poolID, pool.TokenID2) {
						if liquidity > lq {
							matchPool = poolID
						}
					}
				}
				if matchPool == "" {
					poolsLq[pool.PoolID] = liquidity
				} else {
					delete(poolsLq, matchPool)
					poolsLq[pool.PoolID] = liquidity
				}
			}
		}

	}

	poolsLqNew := make(map[string]uint64)
	for p, v := range poolsLq {
		willAdd := false
		isDup := false
		ps := strings.Split(p, "-")
		for p2, v2 := range poolsLq {
			if strings.Contains(p2, ps[0]) && strings.Contains(p2, ps[1]) && p2 != p {
				isDup = true
				if v > v2 {
					willAdd = true
				} else {
					willAdd = false
				}
			}
		}
		if willAdd {
			poolsLqNew[p] = v
		}
		if !isDup {
			poolsLqNew[p] = v
		}
	}

	qualifyPools = "["
	i := 0
	for poolID, _ := range poolsLqNew {
		i++
		if i == len(poolsLqNew) {
			qualifyPools += fmt.Sprintf("\"%v\"", poolID)
		} else {
			qualifyPools += fmt.Sprintf("\"%v\",", poolID)
		}
	}
	qualifyPools += "]"
	return qualifyPools, nil
}
