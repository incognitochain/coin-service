package main

import (
	"flag"
	"io/ioutil"
	"log"
)

var ENABLE_PROFILER bool
var serviceCfg Config
var RESET_FLAG bool

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
	NumberOfBucket        int    `json:"numberofbucket"`
}

// func init() {
// 	serviceCfg.APIPort = DefaultAPIPort
// 	serviceCfg.ChainDataFolder = DefaultChainFolder
// 	serviceCfg.MaxConcurrentOTACheck = DefaultMaxConcurrentOTACheck
// 	serviceCfg.FullnodeAddress = DefaultFullnode
// 	serviceCfg.Mode = DefaultMode
// 	serviceCfg.MongoAddress = DefaultMongoAddress
// 	serviceCfg.MongoDB = DefaultMongoDB
// 	serviceCfg.ChainNetwork = DefaultNetwork
// 	serviceCfg.Highway = DefaultHighway
// }

func readConfigAndArg() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Println(err)
		// return
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
	RESET_FLAG = *argResetDB
	ENABLE_PROFILER = *argProfiler
	serviceCfg = tempCfg
}
