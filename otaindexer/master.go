package otaindexer

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

var upGrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var workers map[string]*worker
var workerLock sync.RWMutex

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys        map[string]*OTAkeyInfo
	AssignedKey map[string]*worker
	TotalKeys   int
}{}

func WorkerRegisterHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
	}
	defer ws.Close()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	workerID := c.Query("id")
	go func() {
		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				log.Println("error read json")
				log.Fatal(err)
			}
			readCh <- msg
		}
	}()

	newWorker := new(worker)
	newWorker.ID = workerID
	newWorker.readCh = readCh
	newWorker.writeCh = writeCh
	err = registerWorker(newWorker)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		msg := <-writeCh
		err := ws.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Println("write:", err)
			return
		}
	}
}

func registerWorker(w *worker) error {
	workerLock.Lock()
	if _, ok := workers[w.ID]; ok {
		return errors.New("workerID already exist")
	}
	workers[w.ID] = w
	workerLock.Unlock()
	return nil
}

var OTAAssignChn chan OTAAssignRequest

func StartWorkerAssigner() {
	loadSubmittedOTAKey()
	OTAAssignChn = make(chan OTAAssignRequest)
	go func() {
		for {
			request := <-OTAAssignChn
			request.Key.IndexerID = shared.ServiceCfg.IndexerID
			err := database.DBSaveSubmittedOTAKeys([]shared.SubmittedOTAKeyData{*request.Key})
			if err != nil {
				go func() {
					request.Respond <- err
				}()
			}
		}
	}()
	timer := time.NewTicker(20 * time.Second)
	for {
		<-timer.C
		Submitted_OTAKey.Lock()
		for key, worker := range Submitted_OTAKey.AssignedKey {
			if worker == nil {
				w, err := chooseWorker()
				if err != nil {
					log.Println(err)
					break
				}
				w.OTAAssigned++
				Submitted_OTAKey.AssignedKey[key] = w
			} else {

			}
		}
		Submitted_OTAKey.Unlock()
	}
}

func chooseWorker() (*worker, error) {
	if len(workers) == 0 {
		return nil, errors.New("no worker available")
	}
	workerLock.RLock()
	var leastOccupiedWorker *worker
	leastOccupiedWorkerKeys := 0

	for _, worker := range workers {
		if time.Since(time.Unix(worker.Heartbeat, 0)) < 40*time.Second {
			if leastOccupiedWorkerKeys == 0 {
				leastOccupiedWorker = worker
				leastOccupiedWorkerKeys = worker.OTAAssigned
				if worker.OTAAssigned == 0 {
					break
				}
			}
			if leastOccupiedWorkerKeys > worker.OTAAssigned {
				leastOccupiedWorkerKeys = worker.OTAAssigned
				leastOccupiedWorker = worker
			}
		}
	}
	workerLock.RUnlock()
	return leastOccupiedWorker, nil
}

func loadSubmittedOTAKey() {
	keys, err := database.DBGetSubmittedOTAKeys(shared.ServiceCfg.IndexerID, 0)
	if err != nil {
		log.Fatalln(err)
	}
	Submitted_OTAKey.Keys = make(map[string]*OTAkeyInfo)
	Submitted_OTAKey.AssignedKey = make(map[string]*worker)
	Submitted_OTAKey.Lock()
	for _, key := range keys {
		pubkey, _, err := base58.Base58Check{}.Decode(key.Pubkey)
		if err != nil {
			log.Fatalln(err)
		}
		keyBytes, _, err := base58.Base58Check{}.Decode(key.OTAKey)
		if err != nil {
			log.Fatalln(err)
		}
		keyBytes = append(keyBytes, pubkey...)
		if len(keyBytes) != 64 {
			log.Fatalln(errors.New("keyBytes length isn't 64"))
		}
		otaKey := shared.OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
		data, err := database.DBGetCoinV2PubkeyInfo(key.Pubkey)
		if err != nil {
			log.Fatalln(err)
		}
		data.OTAKey = key.OTAKey
		k := OTAkeyInfo{
			KeyInfo: data,
			ShardID: int(shardID),
			OTAKey:  key.OTAKey,
			Pubkey:  key.Pubkey,
			keyset:  ks,
		}
		Submitted_OTAKey.Keys[key.Pubkey] = &k
		Submitted_OTAKey.TotalKeys += 1
	}
	Submitted_OTAKey.Unlock()
	log.Printf("Loaded %v keys\n", len(keys))
}
