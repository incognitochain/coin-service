package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

func main() {
	go getData()

	port := os.Getenv("PRICEPORT")
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	err := r.Run("0.0.0.0:" + port)
	if err != nil {
		panic(err)
	}
}

func getData() {
	for {
		time.Sleep(5 * time.Minute)
	}
retry:
	pdecimalURL := os.Getenv("PDECIMAL")
	csv := os.Getenv("COINSERVICE")
	list, err := getExtraTokenInfo(pdecimalURL)
	if err != nil {
		log.Println(err)
		goto retry
	}
	state := getPDEState(csv)

	for _, v := range list {

	}
}

func getPDEState(csvURL string) jsonresult.CurrentPDEState {
retry:
	state := jsonresult.CurrentPDEState{}
	resp, err := http.Get(csvURL + "/getpdestate")
	if err != nil {
		log.Println(err)
		time.Sleep(2 * time.Second)
		goto retry
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(body, &state)
	if err != nil {
		log.Println(err)
		goto retry
	}
	resp.Body.Close()
	// getpdestate
}
