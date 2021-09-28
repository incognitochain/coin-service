package feeestimator

import (
	"math/big"
	"testing"

	"github.com/incognitochain/coin-service/pdexv3/feeestimator/jsonresult"
	"github.com/incognitochain/incognito-chain/common"
)

func TestEstimator(t *testing.T) {
	type TestInput struct {
		sellAmount uint64
		sellToken  string
		tradePath  []string
		feeInPRV   bool
		pdexState  jsonresult.PdexState
	}

	testCases := []struct {
		name   string
		input  TestInput
		output uint64
	}{
		{
			name: "fee paid by sell token - customized fee",
			input: TestInput{
				sellAmount: 15000,
				sellToken:  "ETH",
				tradePath:  []string{"ETH-PRV", "PRV-BTC"},
				feeInPRV:   false,
				pdexState: jsonresult.PdexState{
					Params: jsonresult.PdexParams{
						DefaultFeeRateBPS: 30,
						FeeRateBPS: map[string]uint{
							"ETH-PRV": 20,
						},
					},
				},
			},
			output: 30,
		},
		{
			name: "fee paid by sell token - default fee",
			input: TestInput{
				sellAmount: 15000,
				sellToken:  "ETH",
				tradePath:  []string{"ETH-PRV", "PRV-BTC"},
				feeInPRV:   false,
				pdexState: jsonresult.PdexState{
					Params: jsonresult.PdexParams{
						DefaultFeeRateBPS: 30,
						FeeRateBPS: map[string]uint{
							"PRV-BTC": 20,
						},
					},
				},
			},
			output: 45,
		},
		{
			name: "fee paid by sell token - default amount",
			input: TestInput{
				sellAmount: 34203,
				sellToken:  "ETH",
				tradePath:  []string{"ETH-PRV", "PRV-BTC"},
				feeInPRV:   false,
				pdexState: jsonresult.PdexState{
					Params: jsonresult.PdexParams{
						DefaultFeeRateBPS: 30,
					},
				},
			},
			output: 103,
		},
		{
			name: "fee paid by PRV - no pool PRV",
			input: TestInput{
				sellAmount: 34203,
				sellToken:  "ETH",
				tradePath:  []string{"ETH-PRV", "PRV-BTC"},
				feeInPRV:   true,
				pdexState: jsonresult.PdexState{
					Params: jsonresult.PdexParams{
						DefaultFeeRateBPS: 30,
						FeeRateBPS: map[string]uint{
							"PRV-BTC": 20,
						},
					},
				},
			},
			output: 0,
		},
		{
			name: "fee paid by PRV - has pool PRV",
			input: TestInput{
				sellAmount: 15000,
				sellToken:  "ETH",
				tradePath:  []string{"ETH-USDT", "USDT-BTC"},
				feeInPRV:   true,
				pdexState: jsonresult.PdexState{
					Params: jsonresult.PdexParams{
						DefaultFeeRateBPS:  30,
						PRVDiscountPercent: 25,
					},
					PoolPairs: map[string]*jsonresult.PoolPair{
						"ETH-PRV": {
							State: jsonresult.PoolPairState{
								Token0ID:            "ETH",
								Token1ID:            common.PRVIDStr,
								Token0VirtualAmount: new(big.Int).SetUint64(400),
								Token1VirtualAmount: new(big.Int).SetUint64(300),
								Amplifier:           20000,
							},
						},
					},
				},
			},
			output: 26, // weighted 45 ETH ~ 34 ETH ~ 34 * 3 / 4 PRV
		},
	}

	for _, tc := range testCases {
		if tc.input.feeInPRV {
			fee, _ := EstimatedFeeInPRV(tc.input.sellAmount, tc.input.sellToken, tc.input.tradePath, tc.input.pdexState)
			if fee != tc.output {
				t.Errorf("%s: expected fee %d, got %d", tc.name, tc.output, fee)
			}
		} else {
			if fee, err := EstimateFeeInSellToken(tc.input.sellAmount, tc.input.sellToken, tc.input.tradePath, tc.input.pdexState); err != nil {
				t.Errorf("%s: got error %v", tc.name, err)
			} else if fee != tc.output {
				t.Errorf("%s: got fee %d, want %d", tc.name, fee, tc.output)
			}
		}
	}
}
