package coordinator

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func ConnectToCoordinator(addr string, servicename string, id string, readCh chan []byte, writeCh chan []byte, lostConnFn func()) {
retry:
	u := url.URL{Scheme: "ws", Host: addr, Path: "/connectservice"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"id":      []string{id},
		"service": []string{servicename},
	})
	if err != nil {
		log.Println(err)
		time.Sleep(5 * time.Second)
		goto retry
	}
	defer c.Close()

	done := make(chan struct{})

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
			if lostConnFn != nil {
				lostConnFn()
			}
			goto retry
		case msg := <-writeCh:
			err := c.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				close(done)
				continue
			}
		case <-t.C:
			go func() {
				writeCh <- []byte{1}
			}()
		}

	}
}

func registerService(sv *ServiceConn) error {
	workerLock.Lock()
	if _, ok := workers[w.ID]; ok {
		workerLock.Unlock()
		return errors.New("workerID already exist")
	}
	workers[w.ID] = w
	workerLock.Unlock()
	log.Printf("register worker %v success\n", w.ID)
	return nil
}
