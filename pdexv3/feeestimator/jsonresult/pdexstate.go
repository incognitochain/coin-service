package jsonresult

import (
	"math/big"
)

type PdexParams struct {
	DefaultFeeRateBPS  uint
	FeeRateBPS         map[string]uint
	PRVDiscountPercent uint
	MinPRVReserve      uint64
}

type PoolPairState struct {
	Token0ID            string
	Token1ID            string
	Token0VirtualAmount *big.Int
	Token1VirtualAmount *big.Int
	Amplifier           uint
}

type PoolPair struct {
	State PoolPairState
}

type PdexState struct {
	Params    PdexParams
	PoolPairs map[string]*PoolPair
}
