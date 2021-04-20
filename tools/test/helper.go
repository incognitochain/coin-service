package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/rpcserver"
	"math/big"
)

func ParseResponse(respondInBytes []byte) (*rpcserver.JsonResponse, error) {
	var respond rpcserver.JsonResponse
	err := json.Unmarshal(respondInBytes, &respond)
	if err != nil {
		return nil, err
	}

	if respond.Error != nil{
		return nil, errors.New(fmt.Sprintf("RPC returns an error: %v", respond.Error))
	}

	return &respond, nil
}


func ParseCoinFromJsonResponse(b []byte) ([]shared.ICoinInfo, []*big.Int, error){
	response, err := ParseResponse(b)

	if err != nil{
		return nil, nil, err
	}

	var tmp shared.ListOutputCoins
	err = json.Unmarshal(response.Result, &tmp)
	if err != nil {
		return nil, nil, err
	}

	resultOutCoins := make([]shared.ICoinInfo, 0)
	listOutputCoins := tmp.Outputs
	for _, value := range listOutputCoins {
		for _, outCoin := range value {

			out, _, err := shared.NewCoinFromJsonOutCoin(outCoin)
			if err != nil {
				return nil, nil, err
			}

			resultOutCoins = append(resultOutCoins, out)
		}
	}

	return resultOutCoins, nil, nil
}
