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

		tokenInfoUpdate, err := getInternalTokenPrice()
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
		err = database.DBSaveExtraTokenInfo(extraTokenInfo)
		if err != nil {
			panic(err)
		}
		err = database.DBUpdateTokenInfoPrice(tokenInfoUpdate)
		if err != nil {
			panic(err)
		}

		time.Sleep(updateInterval)
	}
}
