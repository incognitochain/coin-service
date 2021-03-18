package main

import (
	"log"

	// "net/http"

	// _ "net/http/pprof"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	readConfigAndArg()
	err := connectDB()
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", serviceCfg.Mode)
	if serviceCfg.Mode == CHAINSYNCMODE {
		// devframework.TestNetParam.HighwayAddress = "139.162.55.124:9330"
		node := devframework.NewAppNode(serviceCfg.ChainDataFolder, devframework.TestNet2Param, true, false)
		localnode = node
		initCoinService()
	}
	if serviceCfg.Mode == INDEXERMODE {
		devframework.TestNetParam.HighwayAddress = "139.162.55.124:9330"
		node := devframework.NewAppNode(serviceCfg.ChainDataFolder, devframework.TestNetParam, true, false)
		localnode = node
		initCoinService()
		go initOTAIndexingService()
	}
	go startAPIService()
	// http.ListenAndServe("localhost:8091", nil)
	select {}
}
