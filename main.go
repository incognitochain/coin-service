package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/incognitochain/coin-service/apiservice"
	"github.com/incognitochain/coin-service/chainsynker"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/otaindexer"
	"github.com/incognitochain/coin-service/shared"
)

func main() {
	shared.ReadConfigAndArg()
	err := database.ConnectDB(shared.ServiceCfg.MongoDB, shared.ServiceCfg.MongoAddress)
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", shared.ServiceCfg.Mode)
	if shared.ServiceCfg.Mode == shared.FULLMODE {
		chainsynker.InitChainSynker(shared.ServiceCfg)
		go otaindexer.StartOTAIndexingFull()
	}

	if shared.ServiceCfg.Mode == shared.CHAINSYNCMODE {
		chainsynker.InitChainSynker(shared.ServiceCfg)
	}
	if shared.ServiceCfg.Mode == shared.INDEXERMODE {
		go otaindexer.StartWorkerAssigner()
	}
	if shared.ServiceCfg.Mode == shared.WORKERMODE {
		go otaindexer.StartOTAIndexing()
	}
	go apiservice.StartGinService()
	if shared.ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
