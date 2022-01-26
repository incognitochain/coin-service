package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

const (
	add    = "add"
	remove = "remove"
)

type Config struct {
	Mongo    string   `json:"mongo"`
	DBName   string   `json:"dbname"`
	Action   string   `json:"action"`
	DataList []string `json:"datalist"`
}

var cfg Config

func readConfig() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Fatalln(err)
	}

	err = json.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatalln(err)
	}

	if cfg.Action != add && cfg.Action != remove {
		log.Fatalln("invalid action")
	}
	if len(cfg.DataList) == 0 {
		log.Fatalln("invalid datalist")
	}

	return
}
