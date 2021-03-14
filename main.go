package main

import (
	"net/http"
	_ "net/http/pprof"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {

	// acc0, _ = account.NewAccountFromPrivatekey("112t8rnZDRztVgPjbYQiXS7mJgaTzn66NvHD7Vus2SrhSAY611AzADsPFzKjKQCKWTgbkgYrCPo9atvSMoCf9KT23Sc7Js9RKhzbNJkxpJU6")
	// key = []byte{}
	// key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	// if hex.EncodeToString(key) == "d50c6dbb48f7c7a8108299f2e6836168619d15d957623559094cc927676c460a" {
	// 	panic(0)
	// }
	readConfig()
	err := connectDB()
	if err != nil {
		panic(err)
	}
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
