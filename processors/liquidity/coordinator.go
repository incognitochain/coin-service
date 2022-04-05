package liquidity

import (
	"log"
	"time"

	"github.com/incognitochain/coin-service/coordinator"
	"github.com/incognitochain/coin-service/shared"
)

var coordinatorState CoordinatorState

func connectCoordinator(service *coordinator.ServiceConn, coordinatorAddr string) {
	go coordinator.ConnectToCoordinator(coordinatorAddr, service.ServiceName, service.ID, service.ReadCh, service.WriteCh, lostCoordinatorConnection)
	go processMsgFromCoordinator(service.ReadCh)
}

//only called when connection is lost
func lostCoordinatorConnection() {
	log.Println("lost connection to coordinator")
	go coordinator.ConnectToCoordinator(shared.ServiceCfg.CoordinatorAddr, coordinatorState.coordinatorConn.ServiceName, coordinatorState.coordinatorConn.ID, coordinatorState.coordinatorConn.ReadCh, coordinatorState.coordinatorConn.WriteCh, lostCoordinatorConnection)
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
	coordinatorState.pauseService = true
	for {
		if coordinatorState.serviceStatus == "pause" {
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
	coordinatorState.pauseService = false
	for {
		if coordinatorState.serviceStatus == "resume" {
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
	coordinatorState.coordinatorConn.WriteCh <- msg
}

func willPauseOperation() {
	for {
		if coordinatorState.pauseService {
			coordinatorState.serviceStatus = "pause"
			time.Sleep(5 * time.Second)
		} else {
			coordinatorState.serviceStatus = "resume"
			break
		}
	}
}
