package assistant

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/patrickmn/go-cache"
	uuid "github.com/satori/go.uuid"
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

	err = database.DBCreatePNodeDeviceIndex()
	if err != nil {
		panic(err)
	}

	id := uuid.NewV4()
	newServiceConn := coordinator.ServiceConn{
		ServiceName: coordinator.SERVICEGROUP_ASSISTANT,
		ID:          id.String(),
		ReadCh:      make(chan []byte),
		WriteCh:     make(chan []byte),
	}
	coordinatorState.coordinatorConn = &newServiceConn
	coordinatorState.serviceStatus = "pause"
	coordinatorState.pauseService = true
	connectCoordinator(&newServiceConn, shared.ServiceCfg.CoordinatorAddr)

	go calcInternalTokenPrice()
	go getExternalTokenInfo()
	for {
		startTime := time.Now()
		extraTokenInfo, err := getExtraTokenInfo()
		if err != nil {
			panic(err)
		}

		err = database.DBSaveExtraTokenInfo(extraTokenInfo)
		if err != nil {
			panic(err)
		}

		customTokenInfo, err := getCustomTokenInfo()
		if err != nil {
			panic(err)
		}

		err = database.DBSaveCustomTokenInfo(customTokenInfo)
		if err != nil {
			panic(err)
		}

		if time.Since(scanQualifyPools) >= scanQualifyPoolsInterval {
			log.Println("checkPoolQualify")
			qualifyPools, err := checkPoolQualify(extraTokenInfo, customTokenInfo)
			if err != nil {
				panic(err)
			}
			err = database.DBSetQualifyPools(qualifyPools)
			if err != nil {
				panic(err)
			}
			scanQualifyPools = time.Now()
			log.Println("done checkPoolQualify")
		}
		log.Println("finish update info", time.Since(startTime))
		time.Sleep(updateInterval)
	}
}

func calcInternalTokenPrice() {
	for {
		tokenInfoUpdate, err := getInternalTokenPrice()
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

func getExternalTokenInfo() {
	for {
		tokensExternalPrice, err := getBridgeTokenExternalPrice()
		if err != nil {
			panic(err)
		}

		err = database.DBSaveTokenPrice(tokensExternalPrice)
		if err != nil {
			panic(err)
		}

		tokensMkCap, err := getExternalTokenMarketCap()
		if err != nil {
			panic(err)
		}
		err = database.DBSaveTokenMkCap(tokensMkCap)
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
		time.Sleep(updateInterval)
	}
}
