package analyticsquery

import (
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"github.com/incognitochain/coin-service/shared"
	"log"
)

func APIGetPDexV3PairRateHistories(poolid string, period string, intervals string) (*PDexPairRateHistoriesAPIResponse, error){
	var responseBodyData PDexPairRateHistoriesAPIResponse
	_, err := shared.RestyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("poolid", poolid).
		SetQueryParam("period", period).
		SetQueryParam("intervals", intervals).
		SetResult(&responseBodyData).
		Get(shared.ServiceCfg.AnalyticsAPIEndpoint + AnalyticsAPIPath["PDEX_V3_PAIR_RATE_HISTORIES"])
	if err != nil {
		log.Printf("Error getting PDEX_V3_PAIR_RATE_HISTORIES: %s\n", err.Error())
		return nil, err
	}
	spew.Dump(responseBodyData)
	return &responseBodyData, nil
}

func APIGetPDexV3PoolLiquidityHistories(poolId string, period string, intervals string) (*PDexPoolLiquidityHistoriesAPIResponse, error){
	var responseBodyData PDexPoolLiquidityHistoriesAPIResponse
	_, err := shared.RestyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("poolid", poolId).
		SetQueryParam("period", period).
		SetQueryParam("intervals", intervals).
		SetResult(&responseBodyData).
		Get(shared.ServiceCfg.AnalyticsAPIEndpoint + AnalyticsAPIPath["PDEX_V3_POOL_LIQUIDITY_HISTORIES"])
	if err != nil {
		log.Printf("Error getting PDEX_V3_POOL_LIQUIDITY_HISTORIES: %s\n", err.Error())
		return nil, err
	}

	return &responseBodyData, nil
}

func APIGetPDexV3TradingVolume24H(pairName string) (*PDexSummaryDataAPIResponse, error){
	var responseBodyData PDexSummaryDataAPIResponse
	_, err := shared.RestyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("pair", pairName).
		SetResult(&responseBodyData).
		Get(shared.ServiceCfg.AnalyticsAPIEndpoint + AnalyticsAPIPath["PDEX_V3_TRADING_VOLUME_24H"])
	if err != nil {
		log.Printf("Error getting PDEX_V3_TRADING_VOLUME_24H: %s\n", err.Error())
		return nil, err
	}

	return &responseBodyData, nil
}


func APIGetPDexV3PairRateChangesAndVolume24h(poolIDs []string) (map[string]PDexPoolLiquidity, error){
	var responseBodyData PDexPoolRateChangesAPIResponse
	_, err := shared.RestyClient.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]interface{}{
			"poolIds": poolIDs}).
		SetResult(&responseBodyData).
		Post(shared.ServiceCfg.AnalyticsAPIEndpoint + AnalyticsAPIPath["PDEX_V3_TRADING_VOLUME_AND_PAIR_RATE_CHANGES_24H"])
	if err != nil {
		log.Printf("Error getting PDEX_V3_POOL_LIQUIDITY_HISTORIES: %s\n", err.Error())
		return nil, err
	}

	pDexPoolLiquidityMap := make(map[string]PDexPoolLiquidity, 0)
	for key, element := range responseBodyData.Result {
		var poolLiquidity PDexPoolLiquidity
		err = json.Unmarshal(element, &poolLiquidity)

		if err != nil {
			pDexPoolLiquidityMap[key] = poolLiquidity
		}
	}

	return pDexPoolLiquidityMap, nil
}