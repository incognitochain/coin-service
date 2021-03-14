package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

var API_ADDRESS string
var serviceCfg Config

type Config struct {
	APIAddress            string `json:"apiaddress"`
	ChainDataFolder       string `json:"chaindata"`
	MaxConcurrentOTACheck int    `json:"maxconcurrentotacheck"`
	Mode                  string `json:"mode"`
	MongoAddress          string `json:"mongo:"`
}

func init() {
	API_ADDRESS = DefaultAPIAddress
	serviceCfg.APIAddress = DefaultAPIAddress
	serviceCfg.ChainDataFolder = DefaultChainFolder
	serviceCfg.MaxConcurrentOTACheck = MAX_CONCURRENT_OTA_CHECK
	serviceCfg.Mode = DefaultMode
	serviceCfg.MongoAddress = DefaultMongoAddress
}

func readConfig() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Println(err)
		return
	}

	var tempCfg Config
	err = json.Unmarshal(data, &tempCfg)
	if err != nil {
		panic(err)
	}

}
