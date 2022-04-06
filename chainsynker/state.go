package chainsynker

import (
	"sync"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/database"
)

type ChainSyncState struct {
	BlockProcessed     map[int]uint64
	blockProcessedLock sync.RWMutex

	coordinatorConn    *coordinator.ServiceConn
	pauseChainSync     bool
	chainSyncStatus    map[int]string
	chainSyncStatusLck sync.RWMutex
}

func updateState() error {
	currentState.blockProcessedLock.RLock()
	defer currentState.blockProcessedLock.RUnlock()
	result, err := json.Marshal(currentState)
	if err != nil {
		panic(err)
	}
	return database.DBUpdateProcessorState("trade", string(result))
}

func loadState() error {
	result, err := database.DBGetProcessorState("trade")
	if err != nil {
		return err
	}
	if result == nil {
		currentState = ChainSyncState{
			BlockProcessed:  make(map[int]uint64),
			chainSyncStatus: make(map[int]string),
		}
		return nil
	}
	return json.UnmarshalFromString(result.State, &currentState)
}
