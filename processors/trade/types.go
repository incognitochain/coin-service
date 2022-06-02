package trade

import (
	"github.com/incognitochain/coin-service/coordinator"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type State struct {
	LastProcessedObjectID string
}

type CoordinatorState struct {
	coordinatorConn *coordinator.ServiceConn
	pauseService    bool
	serviceStatus   string
}
