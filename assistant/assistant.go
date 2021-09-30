package assistant

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
)

func StartAssistant() {
	log.Println("starting assistant")
	err := database.DBCreateClientAssistantIndex()
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(updateInterval)
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

	}
}
