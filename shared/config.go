package shared

import (
	"flag"
	"io/ioutil"
	"log"
)

var ENABLE_PROFILER bool
var RESET_FLAG bool
var ServiceCfg Config

type Config struct {
	APIPort               int    `json:"apiport"`
	ChainDataFolder       string `json:"chaindata"`
	FullnodeAddress       string `json:"fullnode"`
	MaxConcurrentOTACheck int    `json:"concurrentotacheck"`
	Mode                  string `json:"mode"`
	MongoAddress          string `json:"mongo"`
	MongoDB               string `json:"mongodb"`
	ChainNetwork          string `json:"chainnetwork"`
	Highway               string `json:"highway"`
	IndexerBucketID       int    `json:"bucketid"`
	NumOfShard            int    `json:"numberofshard"`
}

func ReadConfigAndArg() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Fatalln(err)
	}
	var tempCfg Config
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
	if tempCfg.FullnodeAddress == "" {
		tempCfg.FullnodeAddress = DefaultFullnode
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
	if tempCfg.Highway == "" {
		tempCfg.Highway = DefaultHighway
	}
	if tempCfg.NumOfShard == 0 {
		tempCfg.NumOfShard = DefaultNumOfShard
	}
	RESET_FLAG = *argResetDB
	ENABLE_PROFILER = *argProfiler
	ServiceCfg = tempCfg
}
