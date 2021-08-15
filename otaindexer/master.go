package otaindexer

import (
	"context"
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
	"github.com/incognitochain/incognito-chain/privacy/coin"
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
	KeysByShard map[int][]*OTAkeyInfo
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
	go checkMissingIndexCoins()

	for {
		time.Sleep(10 * time.Second)
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
		// log.Println("All keys assigned", len(Submitted_OTAKey.AssignedKey))
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
	Submitted_OTAKey.KeysByShard = make(map[int][]*OTAkeyInfo)
	Submitted_OTAKey.AssignedKey = make(map[string]*worker)
	err = addKeys(keys)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Loaded %v keys\n", len(keys))
}

func addKeys(keys []shared.SubmittedOTAKeyData) error {
	var wg sync.WaitGroup
	for _, key := range keys {
		wg.Add(1)
		go func(k shared.SubmittedOTAKeyData) {
		retry:
			pubkey, _, err := base58.Base58Check{}.Decode(k.Pubkey)
			if err != nil {
				log.Fatalln(err)
			}
			keyBytes, _, err := base58.Base58Check{}.Decode(k.OTAKey)
			if err != nil {
				log.Fatalln(err)
			}
			keyBytes = append(keyBytes, pubkey...)
			if len(keyBytes) != 64 {
				log.Fatalln("keyBytes length isn't 64")
			}
			otaKey := shared.OTAKeyFromRaw(keyBytes)
			ks := &incognitokey.KeySet{}
			ks.OTAKey = otaKey
			shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
			data, err := database.DBGetCoinV2PubkeyInfo(k.Pubkey)
			if err != nil {
				log.Println(err)
				time.Sleep(100 * time.Millisecond)
				goto retry
			}
			data.OTAKey = k.OTAKey
			kInfo := OTAkeyInfo{
				KeyInfo: data,
				ShardID: int(shardID),
				OTAKey:  k.OTAKey,
				Pubkey:  k.Pubkey,
				keyset:  ks,
			}

			Submitted_OTAKey.Lock()
			Submitted_OTAKey.AssignedKey[k.Pubkey] = nil
			Submitted_OTAKey.Keys[k.Pubkey] = &kInfo
			Submitted_OTAKey.KeysByShard[kInfo.ShardID] = append(Submitted_OTAKey.KeysByShard[kInfo.ShardID], &kInfo)
			Submitted_OTAKey.TotalKeys += 1
			Submitted_OTAKey.Unlock()
			wg.Done()
		}(key)
	}
	wg.Wait()
	return nil
}

func ReScanOTAKey(otaKey, pubKey string) error {
	Submitted_OTAKey.RLock()
	defer Submitted_OTAKey.RUnlock()
	if _, ok := Submitted_OTAKey.Keys[pubKey]; !ok {
		return errors.New("wrong indexer")
	}
	pubkey, _, err := base58.Base58Check{}.Decode(pubKey)
	if err != nil {
		return err
	}
	keyBytes, _, err := base58.Base58Check{}.Decode(otaKey)
	if err != nil {
		return err
	}
	keyBytes = append(keyBytes, pubkey...)
	if len(keyBytes) != 64 {
		return errors.New("keyBytes length isn't 64")
	}
	fullOTAKey := shared.OTAKeyFromRaw(keyBytes)
	ks := &incognitokey.KeySet{}
	ks.OTAKey = fullOTAKey
	shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
	data, err := database.DBGetCoinV2PubkeyInfo(pubKey)
	if err != nil {
		return err
	}
	data.OTAKey = otaKey

	for tokenID, coinInfo := range data.CoinIndex {
		if tokenID == common.ConfidentialAssetID.String() {
			coinInfo.LastScanned = 0
			data.CoinIndex[tokenID] = coinInfo
			continue
		}
		coinInfo.Total = uint64(database.DBGetCoinV2OfOTAkeyCount(int(shardID), tokenID, otaKey))
		coinInfo.LastScanned = 0
		txs, err := database.DBGetCountTxByPubkey(pubKey, tokenID, 2)
		if err != nil {
			return err
		}
		data.TotalReceiveTxs[tokenID] = uint64(txs)
		data.CoinIndex[tokenID] = coinInfo
	}
	err = data.Saving()
	if err != nil {
		return err
	}
	doc := bson.M{
		"$set": *data,
	}
	err = database.DBUpdateKeyInfoV2(doc, data, context.Background())
	if err != nil {
		return err
	}
	Submitted_OTAKey.Keys[pubKey].KeyInfo = data

	if w, ok := Submitted_OTAKey.AssignedKey[pubKey]; ok {
		if w.Heartbeat != 0 {
			keyAction := WorkerOTACmd{
				Action: REINDEX,
				Key:    *Submitted_OTAKey.Keys[pubKey],
			}
			keyBytes, err := json.Marshal(keyAction)
			if err != nil {
				log.Fatalln(err)
			}
			w.writeCh <- keyBytes
		}
	}
	return nil
}

func checkMissingIndexCoins() {
	keysNeedRescan := make(map[string]OTAkeyInfo)
	var keysNeedRescanLock sync.Mutex
	for {
		time.Sleep(5 * time.Minute)
		log.Println("start checkMissingIndexCoins")
		start := time.Now()
		Submitted_OTAKey.RLock()
		scanList := make(map[int][]OTAkeyInfo)
		for shardid, keys := range Submitted_OTAKey.KeysByShard {
			for _, v := range keys {
				scanList[shardid] = append(scanList[shardid], OTAkeyInfo{
					Pubkey: v.Pubkey,
					OTAKey: v.OTAKey,
					keyset: v.keyset,
				})
			}
		}
		Submitted_OTAKey.RUnlock()
		keysNeedRescan = make(map[string]OTAkeyInfo)
		_, totalPRV, _, totalToken, err := database.DBGetCoinInfo()
		if err != nil {
			log.Println(err)
			continue
		}
		for shardid, keys := range scanList {
			log.Println("scanList", shardid, len(keys))
			lastIndex := int64(0)
			// check PRV
			for {
				limit := int64(5000)
				log.Println("DBGetUnknownCoinsV22", shardid, common.PRVCoinID.String(), lastIndex, int64(totalPRV[shardid])-10, limit)
				coinList, err := database.DBGetUnknownCoinsV22(shardid, common.PRVCoinID.String(), lastIndex, int64(totalPRV[shardid])-10, limit)
				if err != nil {
					log.Println(err)
					time.Sleep(5 * time.Second)
					continue
				}
				if len(coinList) == 0 {
					break
				}
				lastIndex = int64(coinList[len(coinList)-1].CoinIndex)
				for _, v := range coinList {
					newCoin := new(coin.CoinV2)
					err := newCoin.SetBytes(v.Coin)
					if err != nil {
						panic(err)
					}
					var wg sync.WaitGroup
					for _, key := range keys {
						wg.Add(1)
						go func(c *coin.CoinV2, k OTAkeyInfo) {
							pass, _, _ := doesCoinBelongToKeySet(c, k.keyset, nil, false)
							if pass {
								keysNeedRescanLock.Lock()
								keysNeedRescan[k.OTAKey] = k
								keysNeedRescanLock.Unlock()
							}
							wg.Done()
						}(newCoin, key)
					}
					wg.Wait()
				}
				if int64(len(coinList)) < limit {
					break
				}
			}
			// check token

			lastIndex = int64(0)
			for {
				limit := int64(5000)
				log.Println("DBGetUnknownCoinsV22", shardid, common.ConfidentialAssetID.String(), lastIndex, int64(totalToken[shardid])-10, limit)
				coinList, err := database.DBGetUnknownCoinsV22(shardid, common.ConfidentialAssetID.String(), lastIndex, int64(totalToken[shardid])-10, limit)
				if err != nil {
					log.Println(err)
					time.Sleep(5 * time.Second)
					continue
				}
				if len(coinList) == 0 {
					break
				}
				lastIndex = int64(coinList[len(coinList)-1].CoinIndex)
				for _, v := range coinList {
					newCoin := new(coin.CoinV2)
					err := newCoin.SetBytes(v.Coin)
					if err != nil {
						panic(err)
					}
					var wg sync.WaitGroup
					for _, key := range keys {
						wg.Add(1)
						go func(c *coin.CoinV2, k OTAkeyInfo) {
							pass, _, _ := doesCoinBelongToKeySet(c, k.keyset, nil, false)
							if pass {
								keysNeedRescanLock.Lock()
								keysNeedRescan[k.OTAKey] = k
								keysNeedRescanLock.Unlock()
							}
							wg.Done()
						}(newCoin, key)
					}
					wg.Wait()
				}
				if int64(len(coinList)) < limit {
					break
				}
			}
		}
		log.Println("need to rescan", len(keysNeedRescan), time.Since(start))
		for _, v := range keysNeedRescan {
			err := ReScanOTAKey(v.OTAKey, v.Pubkey)
			if err != nil {
				log.Println("rescan error", v.OTAKey, err)
				continue
			}
		}
	}
}
