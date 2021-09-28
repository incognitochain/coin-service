package assistant

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/incognitochain/coin-service/shared"
)

func getExternalPrice(tokenSymbol string) (float64, error) {
	var price struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}
retry:
	resp, err := http.Get(binancePriceURL + tokenSymbol + "USDT")
	if err != nil {
		log.Println(err)
		goto retry
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(body, &price)
	if err != nil {
		log.Fatalln(err)
	}
	resp.Body.Close()
	value, err := strconv.ParseFloat(price.Price, 32)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func getBridgeTokenExternalPrice() ([]shared.TokenPrice, error) {
	var result []shared.TokenPrice
	bridgeTokens := []shared.TokenInfoData{}

	for _, v := range bridgeTokens {
		getExternalPrice(v.Symbol)
	}
	return result, nil
}
