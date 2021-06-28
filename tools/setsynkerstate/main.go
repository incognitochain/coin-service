package main

import (
	"flag"
	"fmt"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	argNetwork := flag.String("network", "", "set chain network")
	argChainData := flag.String("chaindata", "", "set chain data folder")
	argShardHeight := flag.Uint64("height", 1, "set shard state")
	argShardID := flag.Int("shardid", 0, "set shardID")
	flag.Parse()
	chainNetwork := *argNetwork
	chainData := *argChainData
	var netw devframework.NetworkParam
	switch chainNetwork {
	case "testnet2":
		netw = devframework.TestNet2Param
	case "testnet":
		netw = devframework.TestNetParam
	case "mainnet":
		netw = devframework.MainNetParam
	default:
		panic("unknown network")
	}
	netw.HighwayAddress = "127.0.0.1:1"
	node := devframework.NewAppNode(chainData, netw, true, false, false, false)

	statePrefix := fmt.Sprintf("coin-processed-%v", *argShardID)
	err := node.GetUserDatabase().Put([]byte(statePrefix), []byte(fmt.Sprintf("%v", *argShardHeight)), nil)
	if err != nil {
		panic(err)
	}
}
