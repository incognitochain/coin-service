package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	jsoniter "github.com/json-iterator/go"
)

type APIRespond struct {
	Result jsoniter.RawMessage
	Error  *string
}

func main() {
	argMainnet := flag.Bool("mn", false, "set mainnet")
	flag.Parse()
	csvEndpoint := "http://10.152.183.105:9001/rescanotakey"
	if *argMainnet {
		csvEndpoint = "https://api-coinservice.incognito.org/rescanotakey"
	}
	type keysJSON struct {
		Result []struct {
			Fullkey string `json:"fullkey"`
		}
	}
	fmt.Println("csvEndpoint", csvEndpoint)
	data, err := ioutil.ReadFile("./response.json")
	if err != nil {
		log.Fatalln(err)
	}
	var keys keysJSON
	if data != nil {
		err = json.Unmarshal(data, &keys)
		if err != nil {
			panic(err)
		}
	}
	for _, v := range keys.Result {
		key := v.Fullkey
		reqJSON := struct {
			OTAKey string
		}{
			OTAKey: key,
		}
		reqBytes, err := json.Marshal(reqJSON)
		if err != nil {
			log.Fatalln(err)
		}
		respond, err := makeRequest("POST", csvEndpoint, reqBytes)
		if err != nil {
			log.Println((respond))
			log.Fatalln(err)
		}
		log.Println(key, string(respond))
	}

}

func makeRequest(reqType string, url string, reqBody []byte) ([]byte, error) {
	buf := bytes.NewBuffer(reqBody)
	r, err := http.NewRequest(reqType, url, buf)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Encoding", "gzip")
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
	var apiResp APIRespond
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, err
	}
	if apiResp.Error != nil {
		return nil, errors.New(*apiResp.Error)
	}
	return apiResp.Result, nil
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
