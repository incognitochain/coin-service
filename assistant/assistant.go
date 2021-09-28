package assistant

import (
	"log"
	"time"
)

func StartAssistant() {
	log.Println("starting assistant")
	for {
		time.Sleep(updateInterval)

	}
}
