package feeestimator

import (
	"fmt"
	"math/big"

	"github.com/incognitochain/coin-service/pdexv3/feeestimator/jsonresult"
)

func EstimateFeeInSellToken(
	sellAmount uint64, sellToken string, tradePath []string, pdexState jsonresult.PdexState,
) (uint64, error) {
	if len(tradePath) == 0 {
		return 0, fmt.Errorf("tradePath is empty")
	}
	if sellAmount == 0 {
		return 0, fmt.Errorf("sellAmount is 0")
	}

	// get fee rate of the first pool in trade path
	feeRateBPS := pdexState.Params.DefaultFeeRateBPS
	if _, ok := pdexState.Params.FeeRateBPS[tradePath[0]]; ok {
		feeRateBPS = pdexState.Params.FeeRateBPS[tradePath[0]]
	}

	// find the min feeAmount that feeAmount * BPS >= sellAmount * feeRateBPS
	numerator := new(big.Int).Mul(new(big.Int).SetUint64(sellAmount), new(big.Int).SetUint64(uint64(feeRateBPS)))
	denominator := new(big.Int).SetUint64(BPS)

	fee := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Cmp(big.NewInt(0)) != 0 {
		fee = new(big.Int).Add(fee, big.NewInt(1))
	}

	return fee.Uint64(), nil
}

func EstimatedFeeInPRV(
	sellAmount uint64, sellToken string, tradePath []string, pdexState jsonresult.PdexState,
) (uint64, error) {
	if len(tradePath) == 0 {
		return 0, fmt.Errorf("tradePath is empty")
	}
	if sellAmount == 0 {
		return 0, fmt.Errorf("sellAmount is 0")
	}

	// get fee rate of the first pool in trade path
	feeRateBPS := pdexState.Params.DefaultFeeRateBPS
	if _, ok := pdexState.Params.FeeRateBPS[tradePath[0]]; ok {
		feeRateBPS = pdexState.Params.FeeRateBPS[tradePath[0]]
	}

	// find the min weighted fee that feeAmount * BPS >= sellAmount * feeRateBPS
	numerator := new(big.Int).Mul(new(big.Int).SetUint64(sellAmount), new(big.Int).SetUint64(uint64(feeRateBPS)))
	denominator := new(big.Int).SetUint64(BPS)

	weightedFee := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Cmp(big.NewInt(0)) != 0 {
		weightedFee = new(big.Int).Add(weightedFee, big.NewInt(1))
	}

	rate := getPriceAgainstPRV(sellToken, pdexState)
	if rate[2].Uint64() == 0 {
		return 0, fmt.Errorf("Could not find pair sellToken - PRV")
	}

	// find the min PRV fee amount that prvFeeAmount * rate[0] / rate[1] / (100 - discount) * 100 >= weightedFee
	numerator = new(big.Int).Mul(weightedFee, rate[1])
	numerator = new(big.Int).Mul(numerator, new(big.Int).SetUint64(uint64(100-pdexState.Params.PRVDiscountPercent)))
	denominator = new(big.Int).Mul(rate[0], new(big.Int).SetUint64(uint64(100)))

	prvFee := new(big.Int).Div(numerator, denominator)
	if new(big.Int).Mod(numerator, denominator).Cmp(big.NewInt(0)) != 0 {
		prvFee = new(big.Int).Add(prvFee, big.NewInt(1))
	}

	return prvFee.Uint64(), nil
}

func EstimateTradingFee(
	sellAmount uint64, sellToken string, tradePath []string, pdexState jsonresult.PdexState, useFeeInPRV bool,
) (uint64, error) {
	if useFeeInPRV {
		return EstimatedFeeInPRV(sellAmount, sellToken, tradePath, pdexState)
	} else {
		return EstimateFeeInSellToken(sellAmount, sellToken, tradePath, pdexState)
	}
}