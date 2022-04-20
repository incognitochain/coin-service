package shared

import (
	jsond "encoding/json"
	"math/big"

	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/privacy"
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
	// WaitingContributions map[string]*rawdbv2.Pdexv3Contribution
	PoolPairs         map[string]*PoolPairState         //
	StakingPoolsState map[string]*pdex.StakingPoolState // tokenID -> StakingPoolState
	Params            *pdex.Params
}
type PoolPairState struct {
	ProtocolFees      map[common.Hash]uint64
	StakingPoolFees   map[common.Hash]uint64
	LpFeesPerShare    map[common.Hash]*big.Int
	LmRewardsPerShare map[common.Hash]*big.Int
	State             rawdbv2.Pdexv3PoolPair
	Shares            map[string]*pdex.Share
	Orderbook         struct {
		Orders []*pdex.Order
	}
	MakingVolume map[common.Hash]*MakingVolume // tokenID -> MakingVolume
	OrderRewards map[string]*OrderReward       // nftID -> orderReward
}
type MakingVolume struct {
	Volume map[string]*big.Int // nftID -> amount
}

type OrderRewardDetail struct {
	Receiver privacy.OTAReceiver
	Amount   uint64
}

type OrderReward struct {
	UncollectedRewards map[common.Hash]OrderRewardDetail
	WithdrawnStatus    byte
}

type Pdexv3GetStateRPCResult struct {
	ID     int `json:"Id"`
	Result struct {
		Beacontimestamp      int              `json:"BeaconTimeStamp"`
		Params               jsond.RawMessage `json:"Params"`
		Poolpairs            jsond.RawMessage `json:"PoolPairs"`
		Waitingcontributions struct {
		} `json:"WaitingContributions"`
		Nftids       jsond.RawMessage `json:"NftIDs"`
		Stakingpools jsond.RawMessage `json:"StakingPools"`
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
