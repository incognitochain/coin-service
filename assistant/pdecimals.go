package assistant

import (
	"errors"
	"log"
	"strings"
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
				Decimals           int64         `json:"Decimals"`
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
				ListUnifiedToken   []interface{} `json:"ListUnifiedToken"`
				NetworkID          int           `json:"NetworkID"`
				MovedUnifiedToken  bool          `json:"MovedUnifiedToken"`
				ParentUnifiedID    int           `json:"ParentUnifiedID"`
			}
			Error string `json:"Error"`
		}
		retryTimes := 0
	retry:
		retryTimes++
		if retryTimes > 5 {
			return nil, errors.New("retry reached updatePDecimal")
		}

		_, err := shared.RestyClient.R().
			EnableTrace().
			SetHeader("Accept-Encoding", "gzip").
			SetResult(&decimal).
			Get(shared.ServiceCfg.ExternalDecimals)
		if err != nil {
			log.Printf("Error getting token: %s\n", err.Error())
			time.Sleep(1 * time.Second)
			goto retry
		}

		var result []shared.ExtraTokenInfo
		for _, v := range decimal.Result {
			listChildTkBytes, err := json.Marshal(v.ListChildToken)
			if err != nil {
				return nil, err
			}
			listUnifiedTkBytes, err := json.Marshal(v.ListUnifiedToken)
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
				ListUnifiedToken:   string(listUnifiedTkBytes),
				MovedUnifiedToken:  v.MovedUnifiedToken,
				NetworkID:          v.NetworkID,
				ParentUnifiedID:    v.ParentUnifiedID,
			})
		}
		return result, nil
	}
	return nil, nil
}

func getCustomTokenInfo() ([]shared.CustomTokenInfo, error) {
	if shared.ServiceCfg.ExternalDecimals != "" {
		var decimal struct {
			Result []struct {
				TokenID          string `json:"TokenID" bson:"tokenid"`
				Image            string `json:"Image" bson:"image"`
				IsPrivacy        int    `json:"IsPrivacy" bson:"isprivacy"`
				Name             string `json:"Name" bson:"name"`
				Symbol           string `json:"Symbol" bson:"symbol"`
				OwnerAddress     string `json:"OwnerAddress" bson:"owneraddress"`
				OwnerName        string `json:"OwnerName" bson:"ownername"`
				OwnerEmail       string `json:"OwnerEmail" bson:"owneremail"`
				OwnerWebsite     string `json:"OwnerWebsite" bson:"ownerwebsite"`
				ShowOwnerAddress int    `json:"ShowOwnerAddress" bson:"showowneraddress"`
				Description      string `json:"Description" bson:"description"`
				Verified         bool   `json:"Verified" bson:"verified"`
			}
			Error string `json:"Error"`
		}
		retryTimes := 0
	retry:
		retryTimes++
		if retryTimes > 5 {
			return nil, errors.New("retry reached updatePDecimal")
		}
		urls := strings.Split(shared.ServiceCfg.ExternalDecimals, "/")
		pCustomList := urls[0] + "//" + urls[2] + "/pcustomtoken/list"
		_, err := shared.RestyClient.R().
			EnableTrace().
			SetHeader("Content-Type", "application/json").
			SetHeader("Accept-Encoding", "gzip").
			SetResult(&decimal).
			Get(pCustomList)
		if err != nil {
			log.Printf("Error getting token: %s\n", err.Error())
			time.Sleep(1 * time.Second)
			goto retry
		}

		var result []shared.CustomTokenInfo
		for _, v := range decimal.Result {
			result = append(result, shared.CustomTokenInfo{
				TokenID:          v.TokenID,
				Name:             v.Name,
				Symbol:           v.Symbol,
				Image:            v.Image,
				IsPrivacy:        v.IsPrivacy,
				OwnerAddress:     v.OwnerAddress,
				OwnerName:        v.OwnerName,
				OwnerEmail:       v.OwnerEmail,
				OwnerWebsite:     v.OwnerWebsite,
				ShowOwnerAddress: v.ShowOwnerAddress,
				Description:      v.Description,
				Verified:         v.Verified,
			})
		}
		log.Println("getCustomTokenInfo", len(result))
		return result, nil
	} else {
		log.Println("getCustomTokenInfo", "empty", shared.ServiceCfg.ExternalDecimals, shared.ServiceCfg.ExternalDecimals == "")
	}

	return nil, nil
}
