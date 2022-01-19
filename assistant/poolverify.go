package assistant

import (
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
	for _, pool := range pools {
		_, ok1 := verifiedTks[pool.TokenID1]
		_, ok2 := verifiedTks[pool.TokenID1]
		if ok1 && ok2 {
			q1 := false
			if pool.TokenID1 == common.PRVCoinID.String() || pool.TokenID2 == common.PRVCoinID.String() {

			} else {
				for _, v := range stableCoins {

				}
			}
		}

	}
	return qualifyPools, nil
}
