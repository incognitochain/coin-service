package chainsynker

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/shared"
)

func connectCoordinator(service *coordinator.ServiceConn, coordinatorAddr string) {
	go coordinator.ConnectToCoordinator(coordinatorAddr, service.ServiceName, service.ID, service.ReadCh, service.WriteCh, lostCoordinatorConnection)
	go processMsgFromCoordinator(service.ReadCh)
}

//only called when connection is lost
func lostCoordinatorConnection() {
	log.Println("lost connection to coordinator")
	go coordinator.ConnectToCoordinator(shared.ServiceCfg.CoordinatorAddr, currentState.coordinatorConn.ServiceName, currentState.coordinatorConn.ID, currentState.coordinatorConn.ReadCh, currentState.coordinatorConn.WriteCh, lostCoordinatorConnection)
	log.Println("reconnecting to coordinator")
}

func processMsgFromCoordinator(readCh chan []byte) {
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
	currentState.pauseChainSync = true
	for {
		isAllPaused := true
		currentState.chainSyncStatusLck.RLock()
		for _, status := range currentState.chainSyncStatus {
			if status != "pause" {
				isAllPaused = false
			}
		}
		currentState.chainSyncStatusLck.RUnlock()
		if isAllPaused {
			currentState.coordinatorConn.IsPause = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	action := coordinator.CoordinatorCmd{
		Action: coordinator.ACTION_OPERATION_MODE,
		Data:   "pause",
	}
	actionBytes, _ := json.Marshal(action)
	sendMsgToCoordinator(actionBytes)
}

func resumeOperation() {
	currentState.pauseChainSync = false
	for {
		isAllResume := true
		currentState.chainSyncStatusLck.RLock()
		for _, status := range currentState.chainSyncStatus {
			if status != "resume" {
				isAllResume = false
			}
		}
		currentState.chainSyncStatusLck.RUnlock()
		if isAllResume {
			currentState.coordinatorConn.IsPause = false
			break
		}
		time.Sleep(1 * time.Second)
	}

	action := coordinator.CoordinatorCmd{
		Action: coordinator.ACTION_OPERATION_MODE,
		Data:   "resume",
	}
	actionBytes, _ := json.Marshal(action)
	sendMsgToCoordinator(actionBytes)
}

func sendMsgToCoordinator(msg []byte) {
	currentState.coordinatorConn.WriteCh <- msg
}

func willPauseOperation(chainID int) {
	for {
		if currentState.pauseChainSync {
			currentState.chainSyncStatusLck.Lock()
			if currentState.chainSyncStatus[chainID] != "pause" {
				currentState.chainSyncStatus[chainID] = "pause"
			}
			currentState.chainSyncStatusLck.Unlock()
			time.Sleep(5 * time.Second)
		} else {
			currentState.chainSyncStatusLck.Lock()
			if currentState.chainSyncStatus[chainID] != "resume" {
				currentState.chainSyncStatus[chainID] = "resume"
			}
			currentState.chainSyncStatusLck.Unlock()
			break
		}
	}
}