package shared

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
)

var ENABLE_PROFILER bool
var RESET_FLAG bool
var ServiceCfg Config

type Config struct {
	APIPort               int    `json:"apiport"`
	ChainDataFolder       string `json:"chaindata"`
	EnableChainLog        bool   `json:"chainlog"`
	MaxConcurrentOTACheck int    `json:"concurrentotacheck"`
	Mode                  string `json:"mode"`
	MongoAddress          string `json:"mongo"`
	MongoDB               string `json:"mongodb"`
	ChainNetwork          string `json:"chainnetwork"`
	Highway               string `json:"highway"`
	NumOfShard            int    `json:"numberofshard"`
	IndexerID             int    `json:"indexerid"`
	MasterIndexerAddr     string `json:"masterindexer"`
}

type ConfigJSON struct {
	APIPort               int    `json:"apiport"`
	ChainDataFolder       string `json:"chaindata"`
	EnableChainLog        bool   `json:"chainlog"`
	MaxConcurrentOTACheck int    `json:"concurrentotacheck"`
	Mode                  string `json:"mode"`
	MongoAddress          string `json:"mongo"`
	MongoDB               string `json:"mongodb"`
	ChainNetwork          string `json:"chainnetwork"`
	IndexerID             int    `json:"indexerid"`
	MasterIndexerAddr     string `json:"masterindexer"`
}

func ReadConfigAndArg() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Fatalln(err)
	}
	var tempCfg ConfigJSON
	if data != nil {
		err = json.Unmarshal(data, &tempCfg)
		if err != nil {
			panic(err)
		}
	}

	argProfiler := flag.Bool("profiler", false, "set profiler")
	argResetDB := flag.Bool("resetdb", false, "reset mongodb and resync")
	flag.Parse()
	if tempCfg.APIPort == 0 {
		tempCfg.APIPort = DefaultAPIPort
	}
	if tempCfg.ChainDataFolder == "" {
		tempCfg.ChainDataFolder = DefaultChainFolder
	}
	if tempCfg.MaxConcurrentOTACheck == 0 {
		tempCfg.MaxConcurrentOTACheck = DefaultMaxConcurrentOTACheck
	}
	if tempCfg.Mode == "" {
		tempCfg.Mode = DefaultMode
	}
	if tempCfg.MongoAddress == "" {
		tempCfg.MongoAddress = DefaultMongoAddress
	}
	if tempCfg.MongoDB == "" {
		tempCfg.MongoDB = DefaultMongoDB
	}
	if tempCfg.ChainNetwork == "" {
		tempCfg.ChainNetwork = DefaultNetwork
	}
	pwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	os.Setenv("INCOGNITO_CONFIG_MODE_KEY", "file")
	os.Setenv("INCOGNITO_CONFIG_DIR_KEY", pwd+"/config")
	switch tempCfg.ChainNetwork {
	case "local":
		os.Setenv("INCOGNITO_NETWORK_KEY", "local")
	case "testnet1":
		os.Setenv("INCOGNITO_NETWORK_KEY", "testnet")
		os.Setenv("INCOGNITO_NETWORK_VERSION_KEY", "1")
	case "testnet2":
		os.Setenv("INCOGNITO_NETWORK_KEY", "testnet")
		os.Setenv("INCOGNITO_NETWORK_VERSION_KEY", "2")
	case "mainnet":
		os.Setenv("INCOGNITO_NETWORK_KEY", "mainnet")
	}

	cfg := config.LoadConfig()
	param := config.LoadParam()
	fmt.Println("===========================")
	fmt.Println(cfg.Network())
	fmt.Println(param.ActiveShards)
	fmt.Println("===========================")

	RESET_FLAG = *argResetDB
	ENABLE_PROFILER = *argProfiler

	ServiceCfg.APIPort = tempCfg.APIPort
	ServiceCfg.ChainDataFolder = tempCfg.ChainDataFolder
	ServiceCfg.EnableChainLog = tempCfg.EnableChainLog
	ServiceCfg.MaxConcurrentOTACheck = tempCfg.MaxConcurrentOTACheck
	ServiceCfg.Mode = tempCfg.Mode
	ServiceCfg.MongoAddress = tempCfg.MongoAddress
	ServiceCfg.MongoDB = tempCfg.MongoDB
	ServiceCfg.ChainNetwork = tempCfg.ChainNetwork
	ServiceCfg.Highway = cfg.DiscoverPeersAddress
	ServiceCfg.NumOfShard = param.ActiveShards
	ServiceCfg.IndexerID = tempCfg.IndexerID
	ServiceCfg.MasterIndexerAddr = tempCfg.MasterIndexerAddr

	common.MaxShardNumber = param.ActiveShards
}
