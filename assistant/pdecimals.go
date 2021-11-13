package assistant

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/incognitochain/coin-service/shared"
)

func getExtraTokenInfo() ([]shared.ExtraTokenInfo, error) {
	if shared.ServiceCfg.ExternalDecimals != "" {
		var decimal struct {
			Result []struct {
				TokenID        string        `json:"TokenID"`
				Name           string        `json:"Name"`
				Symbol         string        `json:"Symbol"`
				PSymbol        string        `json:"PSymbol"`
				PDecimals      uint64        `json:"PDecimals"`
				Decimals       uint64        `json:"Decimals"`
				ContractID     string        `json:"ContractID"`
				Status         int           `json:"Status"`
				Type           int           `json:"Type"`
				CurrencyType   int           `json:"CurrencyType"`
				Default        bool          `json:"Default"`
				Verified       bool          `json:"Verified"`
				UserID         int           `json:"UserID"`
				ListChildToken []interface{} `json:"ListChildToken"`
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
		var result []shared.ExtraTokenInfo
		for _, v := range decimal.Result {
			listChildTkBytes, err := json.Marshal(v.ListChildToken)
			if err != nil {
				return nil, err
			}
			result = append(result, shared.ExtraTokenInfo{
				TokenID:        v.TokenID,
				Name:           v.Name,
				Symbol:         v.Symbol,
				PSymbol:        v.PSymbol,
				PDecimals:      v.PDecimals,
				Decimals:       v.Decimals,
				ContractID:     v.ContractID,
				Status:         v.Status,
				Type:           v.Type,
				CurrencyType:   v.CurrencyType,
				Default:        v.Default,
				Verified:       v.Verified,
				UserID:         v.UserID,
				ListChildToken: string(listChildTkBytes),
			})
		}
		return result, nil
	}
	return nil, nil
}
