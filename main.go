package main

import (
	"fmt"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	node := devframework.NewAppNode("fullnode", devframework.TestNetParam, true, false)
	localnode = node
	initCoinService()
	fmt.Println("started daemon in light mode...")
	select {}
}
