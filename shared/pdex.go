package shared

import (
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
)

type CurrentPDEState struct {
	Version         uint
	StateV1         PDEStateV1
	StateV2         PDEStateV2
	BeaconTimeStamp int64
}

type PDEStateV1 struct {
	WaitingContributions map[string]*rawdbv2.PDEContribution
	PDEPoolPairs         map[string]*rawdbv2.PDEPoolForPair
	PDEShares            map[string]uint64
	PDETradingFees       map[string]uint64
}

type PDEStateV2 struct {
	WaitingContributions map[string]*rawdbv2.Pdexv3Contribution
	PoolPairs            map[string]*PoolPairState         //
	StakingPoolsState    map[string]*pdex.StakingPoolState // tokenID -> StakingPoolState
	Params               pdex.Params
}
type PoolPairState struct {
	State     rawdbv2.Pdexv3PoolPair
	Shares    map[string]*pdex.Share
	Orderbook struct {
		Orders []*pdex.Order
	}
}
