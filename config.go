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
	MaxConcurrentOTACheck int    `json:"maxconcurrentotacheck"`
	Mode                  string `json:"mode"`
	MongoAddress          string `json:"mongo"`
	MongoDB               string `json:"mongodb"`
	ChainNetwork          string `json:"chainnetwork"`
	Highway               string `json:"highway"`
}

func init() {
	serviceCfg.APIPort = DefaultAPIPort
	serviceCfg.ChainDataFolder = DefaultChainFolder
	serviceCfg.MaxConcurrentOTACheck = DefaultMaxConcurrentOTACheck
	serviceCfg.FullnodeAddress = DefaultFullnode
	serviceCfg.Mode = DefaultMode
	serviceCfg.MongoAddress = DefaultMongoAddress
	serviceCfg.MongoDB = DefaultMongoDB
	serviceCfg.ChainNetwork = DefaultNetwork
	serviceCfg.Highway = DefaultHighway
}

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

	// overwrite with args
	argMode := flag.String("mode", DefaultMode, "set worker mode")
	argPort := flag.Int("port", DefaultAPIPort, "set worker port")
	argMongo := flag.String("mongo", DefaultMongoAddress, "set mongo address")
	argFullnode := flag.String("fullnode", DefaultFullnode, "set fullnode address")
	argMaxConcurrentOTACheck := flag.Int("maxotacheck", DefaultMaxConcurrentOTACheck, "set MaxConcurrentOTACheck")
	argChain := flag.String("chain", DefaultChainFolder, "set chain folder")
	argDBName := flag.String("dbname", DefaultChainFolder, "set mongodb name")
	argNetwork := flag.String("network", DefaultChainFolder, "set chainetwork name")
	argHighway := flag.String("highway", DefaultChainFolder, "set highway name")
	argProfiler := flag.Bool("profiler", false, "set profiler")
	argResetDB := flag.Bool("resetdb", false, "reset mongodb and resync")
	flag.Parse()
	if tempCfg.APIPort == 0 {
		tempCfg.APIPort = *argPort
	}
	if tempCfg.ChainDataFolder == "" {
		tempCfg.ChainDataFolder = *argChain
	}
	if tempCfg.FullnodeAddress == "" {
		tempCfg.FullnodeAddress = *argFullnode
	}
	if tempCfg.MaxConcurrentOTACheck == 0 {
		tempCfg.MaxConcurrentOTACheck = *argMaxConcurrentOTACheck
	}
	if tempCfg.Mode == "" {
		tempCfg.Mode = *argMode
	}
	if tempCfg.MongoAddress == "" {
		tempCfg.MongoAddress = *argMongo
	}
	if tempCfg.MongoDB == "" {
		tempCfg.MongoDB = *argDBName
	}
	if tempCfg.ChainNetwork == "" {
		tempCfg.ChainNetwork = *argNetwork
	}
	if tempCfg.Highway == "" {
		tempCfg.Highway = *argHighway
	}
	RESET_FLAG = *argResetDB
	ENABLE_PROFILER = *argProfiler
	serviceCfg = tempCfg

	if serviceCfg.MongoAddress == "" || serviceCfg.FullnodeAddress == "" || serviceCfg.MongoDB == "" || serviceCfg.ChainNetwork == "" || serviceCfg.Highway == "" {
		log.Fatalln("MongoAddress & FullnodeAddress & MongoDB & ChainNetwork & Highway can't be empty")
	}
}
