package assistant

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/incognitochain/coin-service/shared"
)

func updatePDecimal() ([]shared.TokenPdecimal, error) {
	if shared.ServiceCfg.ExternalDecimals != "" {
		var decimal struct {
			Result []struct {
				TokenID   string `json:"TokenID"`
				Name      string `json:"Name"`
				Symbol    string `json:"Symbol"`
				PSymbol   string `json:"PSymbol"`
				PDecimals uint64 `json:"PDecimals"`
			}
			Error string `json:"Error"`
		}
		retryTimes := 0
	retry:
		retryTimes++
		if retryTimes > 2 {
			return nil, errors.New("retry reached updatePDecimal")
		}
		resp, err := http.Get(shared.ServiceCfg.ExternalDecimals)
		if err != nil {
			log.Println(err)
			goto retry
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		err = json.Unmarshal(body, &decimal)
		if err != nil {
			log.Println(err)
			goto retry
		}
		resp.Body.Close()
		var result []shared.TokenPdecimal
		for _, v := range decimal.Result {
			result = append(result, shared.TokenPdecimal{
				TokenID:   v.TokenID,
				Name:      v.Name,
				Symbol:    v.Symbol,
				PSymbol:   v.PSymbol,
				PDecimals: v.PDecimals,
			})
		}
		return result, nil
	}
	return nil, nil
}
