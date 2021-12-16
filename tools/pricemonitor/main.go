package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
)

var poolTokens map[string]string
var pDecimal map[string]uint64
var USDTPRVPool string
var tokenInfoListLock sync.Mutex
var tokenInfoList map[string]*ExtraTokenInfo
var prvToUSD float64

func main() {
	log.Println("starting service...")
	poolTokens = make(map[string]string)
	pDecimal = make(map[string]uint64)
	tokenInfoList = make(map[string]*ExtraTokenInfo)
	var err error
	tokenInfoList, err = DBLoadPrice()
	if err != nil {
		log.Println(err)
		tokenInfoList = make(map[string]*ExtraTokenInfo)
	}
	go getData()
	port := os.Getenv("PRICEPORT")
	fmt.Println("PRICEPORT", port)
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{"Content-Type,access-control-allow-origin, access-control-allow-headers"},
	}))
	r.GET("/tokenlist", APIGetTokens)
	err = r.Run("0.0.0.0:" + port)
	if err != nil {
		panic(err)
	}
}

func getData() {
retry:
	pdecimalURL := os.Getenv("PDECIMAL")
	csv := os.Getenv("COINSERVICE")
	fmt.Println("PDECIMAL", pdecimalURL)
	fmt.Println("COINSERVICE", csv)
	list, err := getExtraTokenInfo(pdecimalURL)
	if err != nil {
		log.Println(err)
		goto retry
	}
	tokenInfoListLock.Lock()
	for _, v := range MonitorTokenList {
		for idx, v1 := range list {
			if v == v1.TokenID {
				tokenInfoList[v] = &v1
				pDecimal[v] = v1.PDecimals
				fmt.Println(v, v1.PDecimals)
				break
			}
			if idx == len(list)-1 {
				fmt.Println("idx", v)
			}
		}
	}
	fmt.Println("tokenInfoList", len(MonitorTokenList), len(tokenInfoList))
	tokenInfoListLock.Unlock()
	currentTime := time.Now()
	for {
		state := getPDEState(csv)
		if USDTPRVPool == "" {
			for poolID, _ := range state.PDEPoolPairs {
				if strings.Contains(poolID, BaseToken) && strings.Contains(poolID, common.PRVCoinID.String()) {
					USDTPRVPool = poolID
				}
			}
		}

		tokenInfoListLock.Lock()
		for tokenID, _ := range tokenInfoList {
			if _, ok := poolTokens[tokenID]; !ok {
				for poolID, _ := range state.PDEPoolPairs {
					if tokenID == common.PRVCoinID.String() {
						if strings.Contains(poolID, BaseToken) && strings.Contains(poolID, common.PRVCoinID.String()) {
							poolTokens[tokenID] = poolID
						}
					} else {
						if strings.Contains(poolID, tokenID) && strings.Contains(poolID, common.PRVCoinID.String()) {
							poolTokens[tokenID] = poolID
						}
					}
				}
			}
		}
		pool, ok := state.PDEPoolPairs[poolTokens[common.PRVCoinID.String()]]
		if !ok {
			tokenInfoListLock.Unlock()
			fmt.Println("pool not found", poolTokens[common.PRVCoinID.String()])
			continue
		}
		decimal1 := math.Pow10(int(pDecimal[common.PRVCoinID.String()]))
		decimal2 := math.Pow10(int(pDecimal[BaseToken]))
		x1 := float64(pool.Token1PoolValue*pool.Token2PoolValue) / (float64(pool.Token1PoolValue) + float64(decimal1))
		prvToUSD = -(x1 - float64(pool.Token2PoolValue)) / float64(pool.Token1PoolValue) * (decimal1 / decimal2)

		for tokenID, data := range tokenInfoList {
			var price float64
			if tokenID == common.PRVCoinID.String() {
				price = prvToUSD
			} else {
				price = ProcessPrice(state, tokenID)
			}
			data.PriceUsd = price
			if data.PriceUsd24h == 0 {
				data.LastPiceUpdated = currentTime
				data.PriceUsd24h = price
			}
			if time.Since(data.LastPiceUpdated) >= 24*time.Hour {
				data.LastPiceUpdated = currentTime
				data.PriceUsd24h = price
			}
			if price != 0 {
				data.PercentChange24h = fmt.Sprintf("%.2f", ((price-data.PriceUsd24h)/data.PriceUsd24h)*100)
			}
		}
		tokenInfoListLock.Unlock()
		err = DBStorePrice(tokenInfoList)
		if err != nil {
			panic(err)
		}
		poolTokens = make(map[string]string)
		time.Sleep(10 * time.Second)
	}
}

func getPDEState(csvURL string) *jsonresult.CurrentPDEState {
retry:
	var state struct {
		Result jsonresult.CurrentPDEState
		Error  *string
	}

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
	return &state.Result
}

func DBStorePrice(list map[string]*ExtraTokenInfo) error {
	tokenInfoListLock.Lock()
	defer tokenInfoListLock.Unlock()
	file, _ := json.MarshalIndent(list, "", " ")
	return ioutil.WriteFile("tokens.json", file, 0644)
}

func DBLoadPrice() (map[string]*ExtraTokenInfo, error) {
	var result map[string]*ExtraTokenInfo
	jsonFile, err := os.Open("tokens.json")
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &result)
	return result, err
}

func ProcessPrice(state *jsonresult.CurrentPDEState, tokenID string) float64 {
	if tokenID == BaseToken {
		return 1
	}

	//Token To PRV
	value1 := float64(0)

	if _, ok := poolTokens[tokenID]; !ok {
		fmt.Println(tokenID)
		return 0
	}
	pool := state.PDEPoolPairs[poolTokens[tokenID]]
	decimal1 := math.Pow10(int(pDecimal[common.PRVCoinID.String()]))
	decimal2 := math.Pow10(int(pDecimal[tokenID]))
	x1 := float64(pool.Token1PoolValue*pool.Token2PoolValue) / (float64(pool.Token2PoolValue) + float64(decimal2))
	value1 = math.Abs(float64(pool.Token1PoolValue)-x1) / float64(pool.Token2PoolValue) * (decimal2 / decimal1)
	//PRV to USDT
	value2 := prvToUSD * value1
	return value2
}

func APIGetTokens(c *gin.Context) {
	c.Header("access-control-allow-origin", "*")
	c.Header("Access-Control-Allow-Methods", "PUT, POST, GET, DELETE, OPTIONS")

	tokenInfoListLock.Lock()
	list := []*ExtraTokenInfo{}
	for _, v := range tokenInfoList {
		list = append(list, v)
	}
	tokenInfoListLock.Unlock()
	respond := APIRespond{
		Result: list,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
}
