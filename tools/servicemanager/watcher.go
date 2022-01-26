package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

var notiChan chan string
var notiArray []string
var notiLock sync.Mutex

func startSynkerWatcher() {
	go slackHook()
	for {
		time.Sleep(20 * time.Second)
		query := `{
			"id": 1,
			"jsonrpc": "1.0",
			"method": "getblockchaininfo",
			"params": []
		}`
		r, err := SendPostRequestWithQuery(query)
		if err != nil {
			notiChan <- err.Error()
			continue
		}
		r2, err := ParseResponse(r)
		if err != nil {
			notiChan <- err.Error()
			continue
		}
		var chaininfo jsonresult.GetBlockChainInfoResult
		err = json.Unmarshal(r2.Result, &chaininfo)
		if err != nil {
			notiChan <- err.Error()
			continue
		}
		syncStatus, mongoStatus, err := GetCSVSynkerStatus()
		if err != nil {
			notiChan <- err.Error()
			continue
		}
		if len(syncStatus) != 0 {
			for shardID, info := range chaininfo.BestBlocks {
				if syncStatus[shardID] < info.Height-5 {
					notiChan <- fmt.Sprintf("shard %v is behind %v blocks", shardID, info.Height-syncStatus[shardID])
				}
			}
		}
		if !mongoStatus {
			notiChan <- "mongodb disconnected"
			continue
		}
	}
}

func GetCSVSynkerStatus() (map[int]uint64, bool, error) {
	respond, err := makeRequest("GET", os.Getenv("CSVSYNCKER")+"/health", nil)
	if err != nil {
		return nil, false, err
	}
	var status struct {
		Status string            `json:"status"`
		Mongo  string            `json:"mongo"`
		Chain  map[string]string `json:"chain"`
	}
	err = json.Unmarshal(respond, &status)
	if err != nil {
		return nil, false, err
	}
	syncStatus := make(map[int]uint64)
	if status.Status == "heathy" {
		for chain, v := range status.Chain {
			chainID, _ := strconv.Atoi(chain)
			syncHs := strings.Split(v, "|")
			processH, _ := strconv.Atoi(syncHs[0])
			syncStatus[chainID] = uint64(processH)
		}
	}
	mongoStatus := false
	if status.Mongo == "connected" {
		mongoStatus = true
	}
	return syncStatus, mongoStatus, nil
}
