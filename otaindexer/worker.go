package otaindexer

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/kamva/mgm/v3"
	uuid "github.com/satori/go.uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var assignedOTAKeys = struct {
	sync.RWMutex
	Keys      map[int][]*OTAkeyInfo
	TotalKeys int
}{}
var workerID string

func connectMasterIndexer(addr string, id string, readCh chan []byte, writeCh chan []byte) {
retry:
	u := url.URL{Scheme: "ws", Host: addr, Path: "/indexworker"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"id": []string{id},
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
			cleanAssignedOTA()
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

func processMsgFromMaster(readCh chan []byte, writeCh chan []byte) {
	for {
		msg := <-readCh
		var keyAction WorkerOTACmd
		err := json.Unmarshal(msg, &keyAction)
		if err != nil {
			log.Println(err)
			continue
		}

		assignedOTAKeys.Lock()
		switch keyAction.Action {
		case REINDEX:
			pubkey, _, err := base58.Base58Check{}.Decode(keyAction.Key.Pubkey)
			if err != nil {
				log.Fatalln(err)
			}
			shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
			for _, v := range assignedOTAKeys.Keys[int(shardID)] {
				if v.OTAKey == keyAction.Key.OTAKey {
					for tokenID, coinInfo := range v.KeyInfo.CoinIndex {
						coinInfo.LastScanned = 0
						v.KeyInfo.CoinIndex[tokenID] = coinInfo
					}
				}
			}
		case INDEX:
			pubkey, _, err := base58.Base58Check{}.Decode(keyAction.Key.Pubkey)
			if err != nil {
				log.Fatalln(err)
			}
			keyBytes, _, err := base58.Base58Check{}.Decode(keyAction.Key.OTAKey)
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
			data, err := database.DBGetCoinV2PubkeyInfo(keyAction.Key.Pubkey)
			if err != nil {
				log.Fatalln(err)
			}
			data.OTAKey = keyAction.Key.OTAKey
			k := OTAkeyInfo{
				KeyInfo: data,
				ShardID: int(shardID),
				OTAKey:  keyAction.Key.OTAKey,
				Pubkey:  keyAction.Key.Pubkey,
				keyset:  ks,
			}
			isAlreadyAssigned := false
			for _, v := range assignedOTAKeys.Keys[int(shardID)] {
				if v.OTAKey == keyAction.Key.OTAKey {
					log.Println("key already assign")
					isAlreadyAssigned = true
					break
				}
			}
			if !isAlreadyAssigned {
				assignedOTAKeys.Keys[int(shardID)] = append(assignedOTAKeys.Keys[int(shardID)], &k)
				assignedOTAKeys.TotalKeys += 1
			}
		}
		assignedOTAKeys.Unlock()
		log.Printf("success added key %v for indexing", keyAction.Key.Pubkey)
	}
}

func StartOTAIndexing() {
	log.Println("initiating ota-indexing-service...")
	id := uuid.NewV4()
	workerID = id.String()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	go connectMasterIndexer(shared.ServiceCfg.MasterIndexerAddr, id.String(), readCh, writeCh)
	interval := time.NewTicker(5 * time.Second)
	var coinList []shared.CoinData
	go processMsgFromMaster(readCh, writeCh)
	for {
		<-interval.C
		err := retrieveTokenIDList()
		if err != nil {
			panic(err)
		}
		log.Println("scanning coins...")
		if len(assignedOTAKeys.Keys) == 0 {
			log.Println("len(assignedOTAKeys.Keys) == 0")
			continue
		}
		startTime := time.Now()

		assignedOTAKeys.Lock()
		lastPRVIndex, lastTokenIndex := GetOTAKeyListMinScannedCoinIndex()
		for {
			filteredCoins := make(map[string][]shared.CoinData)
			coinList = GetUnknownCoinsFromDB(lastPRVIndex, lastTokenIndex)
			if len(coinList) == 0 {
				updateOTALastScan(lastPRVIndex, lastTokenIndex)
				break
			}
			filteredCoins, _, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
			if err != nil {
				panic(err)
			}
			updateState(filteredCoins, lastPRVIndex, lastTokenIndex)
		}
		log.Printf("worker/%v finish scanning coins in %v\n", workerID, time.Since(startTime))
		assignedOTAKeys.Unlock()
	}
}

func cleanAssignedOTA() {
	assignedOTAKeys.Lock()
	assignedOTAKeys.Keys = make(map[int][]*OTAkeyInfo)
	assignedOTAKeys.TotalKeys = 0
	assignedOTAKeys.Unlock()
}

func updateState(otaCoinList map[string][]shared.CoinData, lastPRVIndex, lastTokenIndex map[int]uint64) {
	pubkeys := make(map[string]string)

	coinsToUpdate := []shared.CoinData{}
	txToUpdate := make(map[string][]string)
	totalTxs := make(map[string]map[string]map[string]struct{})

	for key, coins := range otaCoinList {
		for _, coin := range coins {
			coin.OTASecret = key
			coinsToUpdate = append(coinsToUpdate, coin)
			txToUpdate[key] = append(txToUpdate[key], coin.TxHash)
			if len(totalTxs[key]) == 0 {
				totalTxs[key] = make(map[string]map[string]struct{})
			}
			if len(totalTxs[key][coin.RealTokenID]) == 0 {
				totalTxs[key][coin.RealTokenID] = make(map[string]struct{})
			}
			totalTxs[key][coin.RealTokenID][coin.TxHash] = struct{}{}
		}
	}

	tokenTxs := []string{}

	for _, txs := range totalTxs {
		for tokenID, txsHash := range txs {
			if tokenID == common.PRVCoinID.String() {
				continue
			}
			for txHash := range txsHash {
				tokenTxs = append(tokenTxs, txHash)
			}
		}
	}
	for key, txs := range totalTxs {
		if txsHash, ok := txs[common.PRVCoinID.String()]; ok {
			for _, hash := range tokenTxs {
				delete(txsHash, hash)
			}
			totalTxs[key][common.PRVCoinID.String()] = txsHash
		}
	}

	for shardID, keyDatas := range assignedOTAKeys.Keys {
		for _, keyData := range keyDatas {
			pubkeys[keyData.OTAKey] = keyData.Pubkey
			if len(keyData.KeyInfo.CoinIndex) == 0 {
				keyData.KeyInfo.CoinIndex = make(map[string]shared.CoinInfo)
			}
			if len(keyData.KeyInfo.TotalReceiveTxs) == 0 {
				keyData.KeyInfo.TotalReceiveTxs = make(map[string]uint64)
			}

			if cd, ok := otaCoinList[keyData.OTAKey]; ok {
				for tokenID, txs := range totalTxs[keyData.OTAKey] {
					keyData.KeyInfo.TotalReceiveTxs[tokenID] += uint64(len(txs))
				}
				sort.Slice(cd, func(i, j int) bool { return cd[i].CoinIndex < cd[j].CoinIndex })
				for _, v := range cd {
					if _, ok := keyData.KeyInfo.CoinIndex[v.RealTokenID]; !ok {
						keyData.KeyInfo.CoinIndex[v.RealTokenID] = shared.CoinInfo{
							Start: v.CoinIndex,
							End:   v.CoinIndex,
							Total: 1,
						}
					} else {
						d := keyData.KeyInfo.CoinIndex[v.RealTokenID]
						if d.Total == 0 {
							d.Start = v.CoinIndex
							d.End = v.CoinIndex
						}
						if d.Start > v.CoinIndex {
							d.Start = v.CoinIndex
						}
						if d.End < v.CoinIndex {
							d.End = v.CoinIndex
						}
						d.Total += 1
						keyData.KeyInfo.CoinIndex[v.RealTokenID] = d
					}
				}
			}

			if d, ok := keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
				if d.LastScanned < lastPRVIndex[shardID] {
					d.LastScanned = lastPRVIndex[shardID]
					keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()] = d
				}
			} else {
				keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()] = shared.CoinInfo{
					LastScanned: lastPRVIndex[shardID],
				}
			}

			if d, ok := keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
				if d.LastScanned < lastTokenIndex[shardID] {
					d.LastScanned = lastTokenIndex[keyData.ShardID]
					keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = d
				}
			} else {
				keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = shared.CoinInfo{
					LastScanned: lastTokenIndex[keyData.ShardID],
				}
			}
		}
	}

	err := mgm.Transaction(func(session mongo.Session, sc mongo.SessionContext) error {
		if len(coinsToUpdate) > 0 {
			log.Println("\n=========================================")
			log.Println("len(coinsToUpdate)", len(coinsToUpdate))
			log.Println("=========================================\n")
			err := database.DBUpdateCoins(coinsToUpdate)
			if err != nil {
				panic(err)
			}
		}

		err := updateSubmittedOTAKey()
		if err != nil {
			panic(err)
		}

		for key, tokenTxs := range totalTxs {
			for tokenID, txList := range tokenTxs {
				txHashs := []string{}
				for txHash := range txList {
					txHashs = append(txHashs, txHash)
				}
				err := database.DBUpdateTxPubkeyReceiver(txHashs, pubkeys[key], tokenID)
				if err != nil {
					panic(err)
				}
			}
		}
		return session.CommitTransaction(sc)
	})
	if err != nil {
		panic(err)
	}

}

func filterCoinsByOTAKey(coinList []shared.CoinData) (map[string][]shared.CoinData, []shared.CoinData, map[int]uint64, map[int]uint64, error) {
	lastPRVIndex := make(map[int]uint64)
	lastTokenIndex := make(map[int]uint64)
	tokenListLock.RLock()
	tokenIDMap := make(map[string]string)
	for k, v := range lastTokenIDMap {
		tokenIDMap[k] = v
	}
	tokenListLock.RUnlock()
	if len(assignedOTAKeys.Keys) > 0 {
		otaCoins := make(map[string][]shared.CoinData)
		var otherCoins []shared.CoinData
		startTime := time.Now()
		var wg sync.WaitGroup
		tempOTACoinsCh := make(chan map[string]shared.CoinData, shared.ServiceCfg.MaxConcurrentOTACheck)
		for idx, c := range coinList {
			wg.Add(1)
			go func(cn shared.CoinData) {
				newCoin := new(coin.CoinV2)
				err := newCoin.SetBytes(cn.Coin)
				if err != nil {
					panic(err)
				}
				pass := false
				tokenID := ""
				for _, keyData := range assignedOTAKeys.Keys[cn.ShardID] {
					if _, ok := keyData.KeyInfo.CoinIndex[cn.TokenID]; ok {
						if cn.CoinIndex < keyData.KeyInfo.CoinIndex[cn.TokenID].LastScanned {
							continue
						}
					}
					pass, tokenID, _ = doesCoinBelongToKeySet(newCoin, keyData.keyset, tokenIDMap)
					if pass {
						cn.RealTokenID = tokenID
						tempOTACoinsCh <- map[string]shared.CoinData{keyData.OTAKey: cn}
						break
					}
				}
				if !pass {
					tempOTACoinsCh <- map[string]shared.CoinData{"nil": cn}
				}
				wg.Done()
			}(c)
			if (idx+1)%shared.ServiceCfg.MaxConcurrentOTACheck == 0 || idx+1 == len(coinList) {
				wg.Wait()
				close(tempOTACoinsCh)
				for k := range tempOTACoinsCh {
					for key, coin := range k {
						if key == "nil" {
							otherCoins = append(otherCoins, coin)
							continue
						}
						otaCoins[key] = append(otaCoins[key], coin)
					}
				}
				// if idx+1 != len(coinList) {
				tempOTACoinsCh = make(chan map[string]shared.CoinData, shared.ServiceCfg.MaxConcurrentOTACheck)
				// }
			}
			if c.TokenID == common.PRVCoinID.String() {
				if _, ok := lastPRVIndex[c.ShardID]; !ok {
					lastPRVIndex[c.ShardID] = c.CoinIndex
				} else {
					if c.CoinIndex > lastPRVIndex[c.ShardID] {
						lastPRVIndex[c.ShardID] = c.CoinIndex
					}
				}
			}
			if c.TokenID == common.ConfidentialAssetID.String() {
				if _, ok := lastTokenIndex[c.ShardID]; !ok {
					lastTokenIndex[c.ShardID] = c.CoinIndex
				} else {
					if c.CoinIndex > lastTokenIndex[c.ShardID] {
						lastTokenIndex[c.ShardID] = c.CoinIndex
					}
				}
			}
		}
		close(tempOTACoinsCh)
		log.Println("len(otaCoins)", len(otaCoins))
		log.Printf("filtered %v coins with %v keys in %v", len(coinList), assignedOTAKeys.TotalKeys, time.Since(startTime))
		log.Println("lastPRVIndex", lastPRVIndex, lastTokenIndex)
		return otaCoins, otherCoins, lastPRVIndex, lastTokenIndex, nil
	}
	return nil, nil, nil, nil, errors.New("no key to scan")
}

func updateSubmittedOTAKey() error {
	docs := []interface{}{}
	KeyInfoList := []*shared.KeyInfoData{}
	for _, keys := range assignedOTAKeys.Keys {
		for _, key := range keys {
			key.KeyInfo.Saving()
			update := bson.M{
				"$set": *key.KeyInfo,
			}
			docs = append(docs, update)
			KeyInfoList = append(KeyInfoList, key.KeyInfo)
		}
	}
	for idx, doc := range docs {
		err := database.DBUpdateKeyInfoV2(doc, KeyInfoList[idx])
		if err != nil {
			assignedOTAKeys.Unlock()
			return err
		}
	}
	return nil
}

func GetOTAKeyListMinScannedCoinIndex() (map[int]uint64, map[int]uint64) {
	minPRVIdx := make(map[int]uint64)
	minTokenIdx := make(map[int]uint64)

	for shardID, keys := range assignedOTAKeys.Keys {
		if _, ok := keys[0].KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
			minPRVIdx[shardID] = keys[0].KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned
		}
		if _, ok := keys[0].KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
			minTokenIdx[shardID] = keys[0].KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
		}
		for _, keyData := range keys {
			if _, ok := keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
				if minPRVIdx[shardID] > keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned {
					minPRVIdx[shardID] = keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned
				}
			} else {
				minPRVIdx[shardID] = 0
			}
			if _, ok := keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
				if minTokenIdx[shardID] > keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned {
					minTokenIdx[shardID] = keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
				}
			} else {
				minTokenIdx[shardID] = 0
			}
		}
	}

	log.Printf("worker/%v minPRVIdx %v %v\n", workerID, minPRVIdx, minTokenIdx)
	return minPRVIdx, minTokenIdx
}

func GetUnknownCoinsFromDB(fromPRVIndex, fromTokenIndex map[int]uint64) []shared.CoinData {
	var result []shared.CoinData
	for shardID, v := range fromPRVIndex {
		if v != 0 {
			v += 1
		}
		coinList, err := database.DBGetUnknownCoinsV2(shardID, common.PRVCoinID.String(), int64(v), 2000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	for shardID, v := range fromTokenIndex {
		if v != 0 {
			v += 1
		}
		coinList, err := database.DBGetUnknownCoinsV2(shardID, common.ConfidentialAssetID.String(), int64(v), 2000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	return result
}

func updateOTALastScan(fromPRVIndex, fromTokenIndex map[int]uint64) {
	newLastScanPRV := make(map[int]uint64)
	newLastScanToken := make(map[int]uint64)

	for shardID, v := range fromPRVIndex {
		coinCount := database.DBGetCoinV2OfShardCount(shardID, common.PRVCoinID.String())
		log.Printf("shard %v count %v prv (lastScanIdx:%v)", shardID, coinCount, v)
		if uint64(coinCount-1) > v {
			newLastScanPRV[shardID] = uint64(coinCount - 1)
		}
	}
	for shardID, v := range fromTokenIndex {
		coinCount := database.DBGetCoinV2OfShardCount(shardID, common.ConfidentialAssetID.String())
		log.Printf("shard %v count %v token (lastScanIdx:%v)", shardID, coinCount, v)
		if uint64(coinCount-1) > v {
			newLastScanToken[shardID] = uint64(coinCount - 1)
		}
	}

	for shardID, v := range newLastScanPRV {
		for _, key := range assignedOTAKeys.Keys[shardID] {
			c := key.KeyInfo.CoinIndex[common.PRVCoinID.String()]
			c.LastScanned = v
			key.KeyInfo.CoinIndex[common.PRVCoinID.String()] = c
		}
	}

	for shardID, v := range newLastScanToken {
		for _, key := range assignedOTAKeys.Keys[shardID] {
			c := key.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]
			c.LastScanned = v
			key.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = c
		}
	}
	err := updateSubmittedOTAKey()
	if err != nil {
		panic(err)
	}
}
