package assistant

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
)

func checkPoolQualify(extraTokenInfo []shared.ExtraTokenInfo) (string, error) {
	var qualifyPools string
	verifiedTks := make(map[string]struct{})
	for _, v := range extraTokenInfo {
		if v.Verified {
			verifiedTks[v.TokenID] = struct{}{}
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

	defaultPools, err := database.DBGetDefaultPool()
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
		_, ok2 := verifiedTks[pool.TokenID1]
		if ok1 && ok2 {
			q1 := false
			tk1Amount, _ := strconv.ParseUint(pool.Token1Amount, 10, 64)
			tk2Amount, _ := strconv.ParseUint(pool.Token2Amount, 10, 64)
			liquidity := uint64(0)
			if pool.TokenID1 == common.PRVCoinID.String() || pool.TokenID2 == common.PRVCoinID.String() {
				q1 = true
				if pool.TokenID1 == common.PRVCoinID.String() {
					liquidity = uint64(float64(tk1Amount) * prvPrice * 2)
				} else {
					liquidity = uint64(float64(tk2Amount) * prvPrice * 2)
				}
			} else {
				for _, v := range stableCoins {
					if pool.TokenID1 == v || pool.TokenID2 == v {
						q1 = true
						if pool.TokenID1 == v {
							liquidity = uint64(float64(tk1Amount) * 2)
						} else {
							liquidity = uint64(float64(tk2Amount) * 2)
						}
						break
					}
				}
			}
			if q1 && liquidity > 0 {
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
	qualifyPools = "["
	i := 0
	for poolID, _ := range poolsLq {
		i++
		if i == len(poolsLq) {
			qualifyPools += fmt.Sprintf("\"%v\"", poolID)
		} else {
			qualifyPools += fmt.Sprintf("\"%v\",", poolID)
		}
	}
	qualifyPools += "]"
	return qualifyPools, nil
}
