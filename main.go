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
		go initOTAIndexingService()
	}
	go startAPIService()
	if ENABLE_PROFILER {
		http.ListenAndServe("localhost:8091", nil)
	}
	select {}
}
