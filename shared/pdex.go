package shared

import (
	jsond "encoding/json"
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


type Pdexv3GetStateRPCResult struct {
	ID     int `json:"Id"`
	Result struct {
		Beacontimestamp      int                    `json:"BeaconTimeStamp"`
		Params               map[string]interface{} `json:"Params"`
		Poolpairs            jsond.RawMessage       `json:"PoolPairs"`
		Waitingcontributions struct {
		} `json:"WaitingContributions"`
		Nftids       map[string]interface{} `json:"NftIDs"`
		Stakingpools map[string]interface{} `json:"StakingPools"`
	} `json:"Result"`
	Error  interface{} `json:"Error"`
	Params []struct {
		Beaconheight int `json:"BeaconHeight"`
	} `json:"Params"`
	Method  string `json:"Method"`
	Jsonrpc string `json:"Jsonrpc"`
}

type Pdexv3PoolPairChild struct {
	PoolID string `json:"PoolID"`
}

type Pdexv3PoolPairWithId struct {
	rawdbv2.Pdexv3PoolPair
	Pdexv3PoolPairChild
}

