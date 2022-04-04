package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ServiceRegisterHandler(c *gin.Context) {
	if state.backupContext != nil {
		c.JSON(200, gin.H{
			"status": "backup is running",
		})
		return
	}
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
	}
	defer ws.Close()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	serviceID := c.Request.Header.Values("id")[0]
	serviceName := c.Request.Header.Values("name")[0]

	newService := new(ServiceConn)
	newService.ID = serviceID
	newService.ServiceName = serviceName
	newService.ReadCh = readCh
	newService.WriteCh = writeCh
	done := make(chan struct{})
	newService.closeCh = done

	go func() {
		for {
			select {
			case <-done:
				removeService(newService)
				close(writeCh)
				ws.Close()
				return
			default:
				_, msg, err := ws.ReadMessage()
				if err != nil {
					log.Println(err)
					close(done)
					return
				}
				var cmd CoordinatorCmd
				err = json.Unmarshal(msg, &cmd)
				if err != nil {
					log.Println(err)
					continue
				}
				switch cmd.Action {
				case ACTION_OPERATION_STATUS:
					if cmd.Data == "pause" {
						newService.IsPause = true
					} else {
						newService.IsPause = false
					}
				}
			}
		}
	}()

	err = registerService(newService)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		select {
		case <-done:
			removeService(newService)
			return
		case msg := <-writeCh:
			fmt.Println("writeCh", string(msg))
			err := ws.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				continue
			}
		}
	}
}

func BackupHandler(c *gin.Context) {
	if state.backupContext != nil {
		c.JSON(200, gin.H{
			"status": "backup is running",
		})
		return
	}
	state.backStatusLock.Lock()
	if time.Since(state.lastSuccessBackupTime) <= 5*time.Minute {
		c.JSON(200, gin.H{
			"status": fmt.Sprintf("backup ran recently, last backup time: %v", state.lastSuccessBackupTime.Format(time.RFC1123Z)),
		})
		state.backStatusLock.Unlock()
		return
	}
	state.backStatusLock.Unlock()
	state.backupContext, state.backupCancelFn = context.WithCancel(context.Background())
	go startBackup()
	c.JSON(200, gin.H{
		"status": "backup started",
	})
}

func BackupStatusHandler(c *gin.Context) {
	if state.backupContext == nil {
		c.JSON(200, gin.H{
			"status": "backup is not running",
		})
		return
	}
	cur, m := state.currentBackupProgress.GetProgressStatus()
	c.JSON(200, gin.H{
		"progress": fmt.Sprintf("%v/%v", cur, m),
	})
	return

}

func CancelBackupHandler(c *gin.Context) {
	if state.backupContext == nil {
		c.JSON(200, gin.H{
			"status": "backup is not running",
		})
		return
	}

	state.backupCancelFn()

	c.JSON(200, gin.H{
		"status": "backup canceled",
	})
	return
}

func GetServiceStatusHandler(c *gin.Context) {

}
