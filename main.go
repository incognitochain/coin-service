package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func main() {
	readConfigAndArg()
	err := connectDB()
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", serviceCfg.Mode)
	if serviceCfg.Mode == TESTMODE {
		initChainSynker()
		go initOTAIndexingService()
	}

	if serviceCfg.Mode == CHAINSYNCMODE {
		initChainSynker()
	}
	if serviceCfg.Mode == INDEXERMODE {
		if serviceCfg.IndexerBucketID == 0 {
			go startBucketAssigner()
		}
		go initOTAIndexingService()
	}
	go startGinService()
	if ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
