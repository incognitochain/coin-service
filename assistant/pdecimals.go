package assistant

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/incognitochain/coin-service/shared"
)

func getExtraTokenInfo() ([]shared.ExtraTokenInfo, error) {
	if shared.ServiceCfg.ExternalDecimals != "" {
		var decimal struct {
			Result []struct {
				TokenID            string        `json:"TokenID"`
				Name               string        `json:"Name"`
				Symbol             string        `json:"Symbol"`
				PSymbol            string        `json:"PSymbol"`
				PDecimals          uint64        `json:"PDecimals"`
				Decimals           uint64        `json:"Decimals"`
				ContractID         string        `json:"ContractID"`
				Status             int           `json:"Status"`
				Type               int           `json:"Type"`
				CurrencyType       int           `json:"CurrencyType"`
				Default            bool          `json:"Default"`
				Verified           bool          `json:"Verified"`
				UserID             int           `json:"UserID"`
				ListChildToken     []interface{} `json:"ListChildToken"`
				PriceUsd           float64       `json:"PriceUsd"`
				PercentChange1h    string        `json:"PercentChange1h"`
				PercentChangePrv1h string        `json:"PercentChangePrv1h"`
				CurrentPrvPool     uint64        `json:"CurrentPrvPool"`
				PricePrv           float64       `json:"PricePrv"`
				Volume24           uint64        `json:"volume24"`
				ParentID           int           `json:"ParentID"`
				Network            string        `json:"Network"`
				OriginalSymbol     string        `json:"OriginalSymbol"`
				LiquidityReward    float64       `json:"LiquidityReward"`
			}
			Error string `json:"Error"`
		}
		retryTimes := 0
	retry:
		retryTimes++
		if retryTimes > 5 {
			return nil, errors.New("retry reached updatePDecimal")
		}
		resp, err := http.Get(shared.ServiceCfg.ExternalDecimals)
		if err != nil {
			log.Println(err)
			time.Sleep(2 * time.Second)
			goto retry
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			time.Sleep(2 * time.Second)
			goto retry
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
				TokenID:            v.TokenID,
				Name:               v.Name,
				Symbol:             v.Symbol,
				PSymbol:            v.PSymbol,
				PDecimals:          v.PDecimals,
				Decimals:           v.Decimals,
				ContractID:         v.ContractID,
				Status:             v.Status,
				Type:               v.Type,
				CurrencyType:       v.CurrencyType,
				Default:            v.Default,
				Verified:           v.Verified,
				UserID:             v.UserID,
				ListChildToken:     string(listChildTkBytes),
				PriceUsd:           v.PriceUsd,
				PercentChange1h:    v.PercentChange1h,
				PercentChangePrv1h: v.PercentChangePrv1h,
				CurrentPrvPool:     v.CurrentPrvPool,
				PricePrv:           v.PricePrv,
				Volume24:           v.Volume24,
				ParentID:           v.ParentID,
				OriginalSymbol:     v.OriginalSymbol,
				LiquidityReward:    v.LiquidityReward,
				Network:            v.Network,
			})
		}
		return result, nil
	}
	return nil, nil
}
