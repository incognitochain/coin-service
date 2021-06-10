package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
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

func main() {
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

func sendSlackNoti(text string) {
	content := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	contentBytes, err := json.Marshal(content)
	if err != nil {
		log.Println(err)
		return
	}
	httpClient := http.DefaultClient
	resp, err := httpClient.Post(os.Getenv("SLACKHOOKCSV"), "application/json", bytes.NewReader(contentBytes))
	if resp.Status != "200" || err != nil {
		log.Println(err)
		body, _ := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}
	defer resp.Body.Close()
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

func SendPostRequestWithQuery(query string) ([]byte, error) {

	var jsonStr = []byte(query)
	req, _ := http.NewRequest("POST", os.Getenv("FULLNODE"), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	} else {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, err
		}
		return body, nil
	}
}

type JsonResponse struct {
	Id      *interface{}    `json:"Id"`
	Result  json.RawMessage `json:"Result"`
	Error   *RPCError       `json:"Error"`
	Params  interface{}     `json:"Params"`
	Method  string          `json:"Method"`
	Jsonrpc string          `json:"Jsonrpc"`
}
type RPCError struct {
	Code       int    `json:"Code,omitempty"`
	Message    string `json:"Message,omitempty"`
	StackTrace string `json:"StackTrace"`

	err error `json:"Err"`
}

func ParseResponse(respondInBytes []byte) (*JsonResponse, error) {
	var respond JsonResponse
	err := json.Unmarshal(respondInBytes, &respond)
	if err != nil {
		return nil, err
	}

	if respond.Error != nil {
		return nil, fmt.Errorf("RPC returns an error: %v", respond.Error)
	}

	return &respond, nil
}

func makeRequest(reqType string, url string, reqBody []byte) ([]byte, error) {
	buf := bytes.NewBuffer(reqBody)
	r, err := http.NewRequest(reqType, url, buf)
	if err != nil {
		return nil, err
	}
	client := new(http.Client)
	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readRespBody(resp)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func readRespBody(resp *http.Response) ([]byte, error) {
	var reader io.ReadCloser
	var err error
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
	default:
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func slackHook() {
	notiChan = make(chan string)
	t := time.NewTicker(30 * time.Second)
	for {
		select {
		case noti := <-notiChan:
			notiLock.Lock()
			notiArray = append(notiArray, noti)
			notiLock.Unlock()
		case <-t.C:
			notiLock.Lock()
			if len(notiArray) > 0 {
				texts := strings.Join(notiArray, "\n")
				sendSlackNoti(texts)
				notiArray = []string{}
			}
			notiLock.Unlock()
		}
	}
}
