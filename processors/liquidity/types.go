package liquidity

import (
	"github.com/incognitochain/coin-service/coordinator"
	jsoniter "github.com/json-iterator/go"
)

type State struct {
	LastProcessedObjectID     string
	LastProcessedPdexV3Height uint64
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type RewardInfo struct {
	RewardPerToken     map[string]uint64
	TokenAmount        map[string]uint64
	RewardReceiveInPRV uint64
	TotalAmountInPRV   uint64
}

type CoordinatorState struct {
	coordinatorConn *coordinator.ServiceConn
	pauseService    bool
	serviceStatus   string
}
