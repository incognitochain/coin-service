package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/incognitochain/coin-service/apiservice"
	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/processors/liquidity"
	"github.com/incognitochain/coin-service/processors/shield"
	"github.com/incognitochain/coin-service/processors/trade"
	"github.com/incognitochain/coin-service/shared"
)

func main() {
	shared.ReadConfigAndArg()
	err := database.ConnectDB(shared.ServiceCfg.MongoDB, shared.ServiceCfg.MongoAddress)
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", shared.ServiceCfg.Mode)
	switch shared.ServiceCfg.Mode {
	// case shared.FULLMODE:
	// 	chainsynker.InitChainSynker(shared.ServiceCfg)
	// 	go otaindexer.StartOTAIndexingFull()
	// 	go apiservice.StartGinService()
	case shared.CHAINSYNCMODE:
		chainsynker.InitChainSynker(shared.ServiceCfg)
		go apiservice.StartGinService()
	case shared.INDEXERMODE:
		go otaindexer.StartWorkerAssigner()
		go apiservice.StartGinService()
	case shared.WORKERMODE:
		go otaindexer.StartOTAIndexing()
	case shared.LIQUIDITYMODE:
		go liquidity.StartProcessor()
	case shared.SHIELDMODE:
		go shield.StartProcessor()
	case shared.TRADEMODE:
		go trade.StartProcessor()
	}

	if shared.ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
