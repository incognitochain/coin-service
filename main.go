package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	readConfig()
	err := connectDB()
	if err != nil {
		panic(err)
	}
	log.Println("service mode:", serviceCfg.Mode)
	if serviceCfg.Mode == INDEXERMODE {
		node := devframework.NewAppNode("fullnode", devframework.TestNetParam, true, false)
		localnode = node
		initCoinService()
		go initOTAIndexingService()
	}
	go startAPIService(DefaultAPIAddress)
	http.ListenAndServe("localhost:8091", nil)
	select {}
}
