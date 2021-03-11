package main

import (
	"encoding/json"
	"io/ioutil"
)

var API_ADDRESS string

func init() {
	API_ADDRESS = DefaultAPIAddress
}

func readConfig() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		panic(err)
	}

	type ConfigJSON struct {
	}
	var cfgJson ConfigJSON
	err = json.Unmarshal(data, &cfgJson)
	if err != nil {
		panic(err)
	}
}
