package coordinator

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func ConnectToCoordinator(addr string, servicename string, id string, readCh chan []byte, writeCh chan []byte, lostConnFn func()) {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/coordinator/connectservice"}
	log.Printf("connecting to coordinator at %s\n", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"id":      []string{id},
		"service": []string{servicename},
	})
	if err != nil {
		log.Println(err)
		lostConnFn()
		return
	}
	defer func() {
		c.Close()
		if lostConnFn != nil {
			lostConnFn()
		}
	}()
	done := make(chan struct{})
	log.Println("connected to coordinator")
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, message, err := c.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					close(done)
					return
				}
				log.Printf("recv: %s", message)
				readCh <- message
			}
		}
	}()
	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-done:
			t.Stop()
			return
		case msg := <-writeCh:
			err := c.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				continue
			}
		case <-t.C:
			//heartbeat
			go func() {
				writeCh <- []byte{1}
			}()
		}
	}
}
