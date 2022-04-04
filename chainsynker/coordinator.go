package chainsynker

import (
	"log"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/shared"
)

var coordinatorConn *coordinator.ServiceConn
var pauseChainSync bool
var chainSyncStatus map[int]string
var chainSyncStatusLck sync.RWMutex

func connectCoordinator(addr string) *coordinator.ServiceConn {
	readCh := make(chan []byte)
	writeCh := make(chan []byte)

	newServiceConn := coordinator.ServiceConn{
		ServiceName: "chainsynker",
		ID:          "1",
		ReadCh:      readCh,
		WriteCh:     writeCh,
	}

	go coordinator.ConnectToCoordinator(addr, newServiceConn.ServiceName, newServiceConn.ID, newServiceConn.ReadCh, newServiceConn.WriteCh, lostCoordinatorConnection)
	go processMsgFromCoordinator(readCh, writeCh)

	return &newServiceConn
}

//only called when connection is lost
func lostCoordinatorConnection() {
	log.Println("lost connection to coordinator")
	go coordinator.ConnectToCoordinator(shared.ServiceCfg.CoordinatorAddr, coordinatorConn.ServiceName, coordinatorConn.ID, coordinatorConn.ReadCh, coordinatorConn.WriteCh, lostCoordinatorConnection)
	log.Println("reconnecting to coordinator")
}

func processMsgFromCoordinator(readCh chan []byte, writeCh chan []byte) {
	for {
		msg := <-readCh
		var action coordinator.CoordinatorCmd
		err := json.Unmarshal(msg, &action)
		if err != nil {
			log.Println(err)
			continue
		}
		switch action.Action {
		case coordinator.ACTION_OPERATION_MODE:
			if action.Data == "pause" {
				pauseOperation()
			} else {
				resumeOperation()
			}
		}
	}
}

func pauseOperation() {
	pauseChainSync = true
}

func resumeOperation() {
	pauseChainSync = false
}

func sendMsgToCoordinator(msg []byte) {
	coordinatorConn.WriteCh <- msg
}

func willPauseOperation(shardID int) {
	for {
		if pauseChainSync {
			chainSyncStatusLck.Lock()
			chainSyncStatus[shardID] = "pause"
			chainSyncStatusLck.Unlock()
		}
		time.Sleep(1 * time.Second)
	}
}
