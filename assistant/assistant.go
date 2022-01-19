package assistant

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/patrickmn/go-cache"
)

var cachedb *cache.Cache

func StartAssistant() {
	cachedb = cache.New(5*time.Minute, 5*time.Minute)
	scanQualifyPools := time.Now().Truncate(scanQualifyPoolsInterval)
	log.Println("starting assistant")
	err := database.DBCreateClientAssistantIndex()
	if err != nil {
		panic(err)
	}
	for {
		tokensPrice, err := getBridgeTokenExternalPrice()
		if err != nil {
			panic(err)
		}

		tokensMkCap, err := getExternalTokenMarketCap()
		if err != nil {
			panic(err)
		}

		pairRanks, err := getPairRanking()
		if err != nil {
			panic(err)
		}

		extraTokenInfo, err := getExtraTokenInfo()
		if err != nil {
			panic(err)
		}

		customTokenInfo, err := getCustomTokenInfo()
		if err != nil {
			panic(err)
		}

		tokenInfoUpdate, err := getInternalTokenPrice(extraTokenInfo)
		if err != nil {
			panic(err)
		}

		err = database.DBSavePairRanking(pairRanks)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveTokenMkCap(tokensMkCap)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveTokenPrice(tokensPrice)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveCustomTokenInfo(customTokenInfo)
		if err != nil {
			panic(err)
		}

		err = database.DBSaveExtraTokenInfo(extraTokenInfo)
		if err != nil {
			panic(err)
		}

		err = database.DBUpdateTokenInfoPrice(tokenInfoUpdate)
		if err != nil {
			panic(err)
		}
		if time.Since(scanQualifyPools) >= scanQualifyPoolsInterval {
			qualifyPools, err := checkPoolQualify(extraTokenInfo)
			if err != nil {
				panic(err)
			}
			err = database.DBSetQualifyPool(qualifyPools)
			if err != nil {
				panic(err)
			}
			scanQualifyPools = time.Now()
		}

		time.Sleep(updateInterval)
	}
}
