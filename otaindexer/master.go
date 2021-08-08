package otaindexer

import (
	"errors"
	"fmt"
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
	jsoniter "github.com/json-iterator/go"
	"go.mongodb.org/mongo-driver/bson"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

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

const (
	REINDEX = "reindex"
	INDEX   = "index"
)

func init() {
	assignedOTAKeys.Keys = make(map[int][]*OTAkeyInfo)
	workers = make(map[string]*worker)
	OTAAssignChn = make(chan OTAAssignRequest)
}

func GetWorkerStat(c *gin.Context) {
	workerLock.RLock()
	defer workerLock.RUnlock()
	type APIRespond struct {
		Result interface{}
		Error  *string
	}
	stats := make(map[string]uint64)
	for _, v := range workers {
		stats[v.ID] = uint64(v.OTAAssigned)
	}
	respond := APIRespond{
		Result: stats,
		Error:  nil,
	}
	c.JSON(http.StatusOK, respond)
	return
}

func WorkerRegisterHandler(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("error get connection")
		log.Fatal(err)
	}
	defer ws.Close()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	workerID := c.Request.Header.Values("id")[0]

	newWorker := new(worker)
	newWorker.ID = workerID
	newWorker.readCh = readCh
	newWorker.writeCh = writeCh
	newWorker.Heartbeat = time.Now().Unix()
	done := make(chan struct{})
	newWorker.closeCh = done

	go func() {
		for {
			select {
			case <-done:
				newWorker.OTAAssigned = 0
				newWorker.Heartbeat = 0
				removeWorker(newWorker)
				ws.Close()
				return
			default:
				_, msg, err := ws.ReadMessage()
				if err != nil {
					log.Println(err)
					close(done)
					return
				}
				if len(msg) == 1 && msg[0] == 1 {
					newWorker.Heartbeat = time.Now().Unix()
					continue
				}
			}
		}
	}()

	err = registerWorker(newWorker)
	if err != nil {
		log.Println(err)
		return
	}
	for {
		select {
		case <-done:
			newWorker.OTAAssigned = 0
			newWorker.Heartbeat = 0
			removeWorker(newWorker)
			return
		case msg := <-writeCh:
			err := ws.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				log.Println("write:", err)
				continue
			}
		}
	}
}

func registerWorker(w *worker) error {
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

func removeWorker(w *worker) error {
	workerLock.Lock()
	if _, ok := workers[w.ID]; !ok {
		workerLock.Unlock()
		return errors.New("workerID not exist")
	}
	delete(workers, w.ID)
	workerLock.Unlock()
	log.Printf("remove worker %v success\n", w.ID)
	return nil
}

var OTAAssignChn chan OTAAssignRequest

func StartWorkerAssigner() {
	loadSubmittedOTAKey()
	go func() {
		for {
			request := <-OTAAssignChn
			Submitted_OTAKey.RLock()
			if _, ok := Submitted_OTAKey.Keys[request.Key.Pubkey]; ok {
				go func() {
					request.Respond <- fmt.Errorf("key %v already exist", request.Key.Pubkey)
				}()
				Submitted_OTAKey.RUnlock()
				continue
			}
			Submitted_OTAKey.RUnlock()

			request.Key.IndexerID = shared.ServiceCfg.IndexerID
			err := database.DBSaveSubmittedOTAKeys([]shared.SubmittedOTAKeyData{*request.Key})
			if err != nil {
				go func() {
					request.Respond <- err
				}()
				continue
			}
			err = addKeys([]shared.SubmittedOTAKeyData{*request.Key})
			if err != nil {
				go func() {
					request.Respond <- err
				}()
				continue
			}
			go func() {
				request.Respond <- nil
			}()
		}
	}()
	timer := time.NewTicker(10 * time.Second)
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
				log.Printf("assign %v to worker %v", key, w.ID)
				keyAction := WorkerOTACmd{
					Action: INDEX,
					Key:    *Submitted_OTAKey.Keys[key],
				}
				keyBytes, err := json.Marshal(keyAction)
				if err != nil {
					log.Fatalln(err)
				}
				w.writeCh <- keyBytes
			} else {
				if worker.Heartbeat == 0 {
					keyinfo, err := database.DBGetCoinV2PubkeyInfo(Submitted_OTAKey.Keys[key].Pubkey)
					if err != nil {
						log.Fatalln(err)
					}
					Submitted_OTAKey.Keys[key].KeyInfo = keyinfo
					w, err := chooseWorker()
					if err != nil {
						log.Println(err)
						break
					}
					w.OTAAssigned++
					Submitted_OTAKey.AssignedKey[key] = w
					log.Printf("re-assign %v to worker %v", key, w.ID)
					keyAction := WorkerOTACmd{
						Action: INDEX,
						Key:    *Submitted_OTAKey.Keys[key],
					}
					keyBytes, err := json.Marshal(keyAction)
					if err != nil {
						log.Fatalln(err)
					}
					w.writeCh <- keyBytes
				}
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
		if time.Since(time.Unix(worker.Heartbeat, 0)) <= 40*time.Second {
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
	if leastOccupiedWorker == nil {
		return nil, errors.New("no worker available")
	}
	return leastOccupiedWorker, nil
}

func loadSubmittedOTAKey() {
	keys, err := database.DBGetSubmittedOTAKeys(shared.ServiceCfg.IndexerID, 0)
	if err != nil {
		log.Fatalln(err)
	}
	Submitted_OTAKey.Keys = make(map[string]*OTAkeyInfo)
	Submitted_OTAKey.AssignedKey = make(map[string]*worker)
	err = addKeys(keys)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Loaded %v keys\n", len(keys))
}

func addKeys(keys []shared.SubmittedOTAKeyData) error {
	Submitted_OTAKey.Lock()
	defer Submitted_OTAKey.Unlock()
	for _, key := range keys {
		pubkey, _, err := base58.Base58Check{}.Decode(key.Pubkey)
		if err != nil {
			return err
		}
		keyBytes, _, err := base58.Base58Check{}.Decode(key.OTAKey)
		if err != nil {
			return err
		}
		keyBytes = append(keyBytes, pubkey...)
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		otaKey := shared.OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
		data, err := database.DBGetCoinV2PubkeyInfo(key.Pubkey)
		if err != nil {
			return err
		}
		data.OTAKey = key.OTAKey
		k := OTAkeyInfo{
			KeyInfo: data,
			ShardID: int(shardID),
			OTAKey:  key.OTAKey,
			Pubkey:  key.Pubkey,
			keyset:  ks,
		}
		Submitted_OTAKey.AssignedKey[key.Pubkey] = nil
		Submitted_OTAKey.Keys[key.Pubkey] = &k
		Submitted_OTAKey.TotalKeys += 1
	}
	return nil
}

func ReScanOTAKey(key shared.SubmittedOTAKeyData) error {
	Submitted_OTAKey.Lock()
	defer Submitted_OTAKey.Unlock()
	if _, ok := Submitted_OTAKey.Keys[key.Pubkey]; !ok {
		return errors.New("wrong indexer")
	}
	pubkey, _, err := base58.Base58Check{}.Decode(key.Pubkey)
	if err != nil {
		return err
	}
	keyBytes, _, err := base58.Base58Check{}.Decode(key.OTAKey)
	if err != nil {
		return err
	}
	keyBytes = append(keyBytes, pubkey...)
	if len(keyBytes) != 64 {
		return errors.New("keyBytes length isn't 64")
	}
	otaKey := shared.OTAKeyFromRaw(keyBytes)
	ks := &incognitokey.KeySet{}
	ks.OTAKey = otaKey
	shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
	data, err := database.DBGetCoinV2PubkeyInfo(key.Pubkey)
	if err != nil {
		return err
	}
	data.OTAKey = key.OTAKey
	if w, ok := Submitted_OTAKey.AssignedKey[key.Pubkey]; !ok {
		return errors.New("key not assign yet")
	} else {
		for tokenID, coinInfo := range data.CoinIndex {
			if tokenID == common.ConfidentialAssetID.String() {
				coinInfo.LastScanned = 0
				data.CoinIndex[tokenID] = coinInfo
				continue
			}
			coinInfo.Total = uint64(database.DBGetCoinV2OfOTAkeyCount(int(shardID), tokenID, key.OTAKey))
			coinInfo.LastScanned = 0
			txs, err := database.DBGetCountTxByPubkey(key.Pubkey, tokenID, 2)
			if err != nil {
				return err
			}
			data.TotalReceiveTxs[tokenID] = uint64(txs)
			data.CoinIndex[tokenID] = coinInfo
		}
		err := data.Saving()
		if err != nil {
			return err
		}
		doc := bson.M{
			"$set": *data,
		}
		err = database.DBUpdateKeyInfoV2(doc, data)
		if err != nil {
			return err
		}
		Submitted_OTAKey.Keys[key.Pubkey].KeyInfo = data

		keyAction := WorkerOTACmd{
			Action: REINDEX,
			Key:    *Submitted_OTAKey.Keys[key.Pubkey],
		}
		keyBytes, err := json.Marshal(keyAction)
		if err != nil {
			log.Fatalln(err)
		}
		w.writeCh <- keyBytes
	}

	return nil
}
