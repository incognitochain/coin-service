package chainsynker

import (
	"log"

	"github.com/incognitochain/coin-service/coordinator"
)

func connectCoordinator(addr string) {
	readCh := make(chan []byte)
	writeCh := make(chan []byte)

	go coordinator.ConnectToCoordinator(addr, "chainsynker", "1", readCh, writeCh, lostCoordinatorConnection)
	go processMsgFromCoordinator(readCh, writeCh)

}

func lostCoordinatorConnection() {
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
		// case RUN:
		// 	willRun = true
		// case REINDEX:

		}
	}
}
