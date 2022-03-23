package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/incognitochain/coin-service/analyticdb"
	"github.com/incognitochain/coin-service/apiservice"
	"github.com/incognitochain/coin-service/assistant"
	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/processors/liquidity"
	"github.com/incognitochain/coin-service/processors/shield"
	"github.com/incognitochain/coin-service/processors/trade"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/wallet"
)

func main() {
	shared.ReadConfigAndArg()
	err := database.ConnectDB(shared.ServiceCfg.MongoDB, shared.ServiceCfg.MongoAddress)
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", shared.ServiceCfg.Mode)
	err = wallet.InitPublicKeyBurningAddressByte()
	if err != nil {
		panic(err)
	}
	switch shared.ServiceCfg.Mode {
	// case shared.FULLMODE:
	// 	chainsynker.InitChainSynker(shared.ServiceCfg)
	// 	go otaindexer.StartOTAIndexingFull()
	// 	go apiservice.StartGinService()
	case shared.QUERYMODE:
		err = analyticdb.ConnectDB(shared.ServiceCfg.AnalyticAddress)
		if err != nil {
			panic(err)
		}
		go apiservice.StartGinService()
	case shared.CHAINSYNCMODE:
		chainsynker.InitChainSynker(shared.ServiceCfg)
		go apiservice.StartGinService()
	case shared.INDEXERMODE:
		go otaindexer.StartWorkerAssigner()
		go apiservice.StartGinService()
	case shared.WORKERMODE:
		go otaindexer.StartOTAIndexing()
	case shared.LIQUIDITYMODE:
		err = analyticdb.ConnectDB(shared.ServiceCfg.AnalyticAddress)
		if err != nil {
			panic(err)
		}
		go liquidity.StartProcessor()
	case shared.SHIELDMODE:
		err = analyticdb.ConnectDB(shared.ServiceCfg.AnalyticAddress)
		if err != nil {
			panic(err)
		}
		go shield.StartProcessor()
	case shared.TRADEMODE:
		err = analyticdb.ConnectDB(shared.ServiceCfg.AnalyticAddress)
		if err != nil {
			panic(err)
		}
		go trade.StartProcessor()
	case shared.ASTMODE:
		go assistant.StartAssistant()
	}

	if shared.ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
