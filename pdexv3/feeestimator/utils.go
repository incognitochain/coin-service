package feeestimator

import (
	"encoding/json"
	"math/big"

	"github.com/incognitochain/coin-service/pdexv3/feeestimator/jsonresult"
	"github.com/incognitochain/incognito-chain/common"
)

func getPriceAgainstPRV(sellToken string, pdexState jsonresult.PdexState) [3]*big.Int {
	result := [3]*big.Int{new(big.Int).SetInt64(0), new(big.Int).SetInt64(0), new(big.Int).SetInt64(0)}
	for _, pair := range pdexState.PoolPairs {
		var tokenID string
		virtualTokenReserve := big.NewInt(0)
		virtualPRVReserve := big.NewInt(0)
		if pair.State.Token0ID == common.PRVIDStr {
			tokenID = pair.State.Token1ID
			virtualTokenReserve.Set(pair.State.Token1VirtualAmount)
			virtualPRVReserve.Set(pair.State.Token0VirtualAmount)
		} else if pair.State.Token1ID == common.PRVIDStr {
			tokenID = pair.State.Token0ID
			virtualTokenReserve.Set(pair.State.Token0VirtualAmount)
			virtualPRVReserve.Set(pair.State.Token1VirtualAmount)
		}

		if tokenID != sellToken {
			continue
		}

		// compare normalized PRV reserve against minPRVReserve -> compare PRV vReserve * baseAmplifer against minPRVReserve * amplifier rate
		temp1 := big.NewInt(0).Mul(virtualPRVReserve, big.NewInt(BaseAmplifier))
		temp2 := big.NewInt(0).Mul(big.NewInt(0).SetUint64(pdexState.Params.MinPRVReserveTradingRate), big.NewInt(0).SetUint64(uint64(pair.State.Amplifier)))
		if temp1.Cmp(temp2) < 0 {
			continue
		}

		normalizedLiquidity := big.NewInt(0).Mul(virtualTokenReserve, virtualPRVReserve)
		normalizedLiquidity.Mul(normalizedLiquidity, big.NewInt(BaseAmplifier))
		normalizedLiquidity.Div(normalizedLiquidity, big.NewInt(0).SetUint64(uint64(pair.State.Amplifier)))
		normalizedLiquidity.Mul(normalizedLiquidity, big.NewInt(BaseAmplifier))
		normalizedLiquidity.Div(normalizedLiquidity, big.NewInt(0).SetUint64(uint64(pair.State.Amplifier)))

		isChosenPool := false
		if result[1].IsUint64() && result[1].Uint64() == 0 {
			isChosenPool = true
		} else {
			liqCmp := normalizedLiquidity.Cmp(result[2])
			if liqCmp == 1 {
				// for each pair of token/PRV, choose pool to maximize normalized liquidity
				isChosenPool = true
			} else if liqCmp == 0 {
				// handle equalities explicitly to keep result deterministic regardless of map traversing order
				// break equality with direct rate comparison (token / PRV)
				temp := result
				theirVirtualTokenReserve := temp[0]
				theirVirtualPRVReserve := temp[1]
				rateCmp := big.NewInt(0).Mul(virtualTokenReserve, theirVirtualPRVReserve).
					Cmp(big.NewInt(0).Mul(theirVirtualTokenReserve, virtualPRVReserve))
				if rateCmp == 1 {
					// when token/PRV pools tie in liquidity, maximize token/PRV rate to the benefit of current user
					isChosenPool = true
				}
			}
		}

		if isChosenPool {
			result = [3]*big.Int{virtualTokenReserve, virtualPRVReserve, normalizedLiquidity}
		}
	}
	return result
}

func GetPdexv3PoolDataFromRawRPCResult(pdexParamRaw json.RawMessage, pdexPoolPairsRaw json.RawMessage) (*jsonresult.PdexState, error) {
	var pdexParams jsonresult.PdexParams
	var poolPairs map[string]*jsonresult.PoolPair
	err := json.Unmarshal(pdexParamRaw, &pdexParams)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(pdexPoolPairsRaw, &poolPairs)

	pdexState := jsonresult.PdexState{
		Params:    pdexParams,
		PoolPairs: poolPairs,
	}

	return &pdexState, nil
}
