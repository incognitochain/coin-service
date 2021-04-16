package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// @title Swagger Coinservice API
// @version 1.0
// @description coinservice api

// @license.name MIT

// @BasePath /t
func main() {
	shared.ReadConfigAndArg()
	err := database.ConnectDB()
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", shared.ServiceCfg.Mode)
	if shared.ServiceCfg.Mode == shared.TESTMODE {
		initChainSynker()
		go initOTAIndexingService()
	}

	if shared.ServiceCfg.Mode == shared.CHAINSYNCMODE {
		initChainSynker()
	}
	if shared.ServiceCfg.Mode == shared.INDEXERMODE {
		if shared.ServiceCfg.IndexerBucketID == 0 {
			go startBucketAssigner()
		}
		go initOTAIndexingService()
	}
	go startGinService()
	if shared.ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
