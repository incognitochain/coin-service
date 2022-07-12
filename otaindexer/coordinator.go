package otaindexer

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/shared"
)

var coordinatorState CoordinatorState

func connectCoordinator(service *coordinator.ServiceConn, coordinatorAddr string) {
	go coordinator.ConnectToCoordinator(coordinatorAddr, service, lostCoordinatorConnection)
	go processMsgFromCoordinator(service.ReadCh)
}

//only called when connection is lost
func lostCoordinatorConnection() {
	log.Println("lost connection to coordinator")
	go coordinator.ConnectToCoordinator(shared.ServiceCfg.CoordinatorAddr, coordinatorState.coordinatorConn, lostCoordinatorConnection)
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
			switch action.Data {
			case "pause":
				pauseOperation()
			case "resume":
				resumeOperation()
			case "get":
				getOperationStatus()
			}
		}
	}
}

func pauseOperation() {
	Submitted_OTAKey.Lock()
	workerLock.Lock()
	defer func() {
		Submitted_OTAKey.Unlock()
		workerLock.Unlock()
	}()
	coordinatorState.pauseService = true
	for {
		if coordinatorState.serviceStatus == "pause" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	action := coordinator.CoordinatorCmd{
		Action: coordinator.ACTION_OPERATION_STATUS,
		Data:   "pause",
	}
	actionBytes, _ := json.Marshal(action)
	sendMsgToCoordinator(actionBytes)
}

func resumeOperation() {
	Submitted_OTAKey.Lock()
	workerLock.Lock()
	defer func() {
		Submitted_OTAKey.Unlock()
		workerLock.Unlock()
	}()
	coordinatorState.pauseService = false
	for {
		if coordinatorState.serviceStatus == "resume" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	action := coordinator.CoordinatorCmd{
		Action: coordinator.ACTION_OPERATION_STATUS,
		Data:   "resume",
	}
	actionBytes, _ := json.Marshal(action)
	sendMsgToCoordinator(actionBytes)
}

func getOperationStatus() {
	action := coordinator.CoordinatorCmd{
		Action: coordinator.ACTION_OPERATION_STATUS,
		Data:   coordinatorState.serviceStatus,
	}
	actionBytes, _ := json.Marshal(action)
	sendMsgToCoordinator(actionBytes)
}

func sendMsgToCoordinator(msg []byte) {
	coordinatorState.coordinatorConn.WriteCh <- msg
}

func willPauseOperation() {
	for {
		if coordinatorState.pauseService {
			coordinatorState.serviceStatus = "pause"
			time.Sleep(5 * time.Second)
		} else {
			coordinatorState.serviceStatus = "resume"
			log.Println("service resumed")
			break
		}
	}
}
