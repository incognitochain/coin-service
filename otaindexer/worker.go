package otaindexer

import (
	"encoding/json"
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
	uuid "github.com/satori/go.uuid"
	"go.mongodb.org/mongo-driver/bson"
)

var assignedOTAKeys = struct {
	sync.RWMutex
	Keys      map[int][]*OTAkeyInfo
	TotalKeys int
}{}

func connectMasterIndexer(addr string, id string, readCh chan []byte, writeCh chan []byte) {
retry:
	u := url.URL{Scheme: "ws", Host: addr, Path: "/indexworker"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"id": []string{id},
	})
	if err != nil {
		log.Println(err)
		goto retry
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			readCh <- message
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
				return
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
		var key OTAkeyInfo
		err := json.Unmarshal(msg, &key)
		if err != nil {
			log.Println(err)
			continue
		}
		assignedOTAKeys.Lock()
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
		assignedOTAKeys.Keys[int(shardID)] = append(assignedOTAKeys.Keys[int(shardID)], &k)
		assignedOTAKeys.TotalKeys += 1
		assignedOTAKeys.Unlock()
		log.Printf("success added key %v for indexing", key.Pubkey)
	}
}

func StartOTAIndexing() {
	log.Println("initiating ota-indexing-service...")
	id := uuid.NewV4()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	go connectMasterIndexer(shared.ServiceCfg.MasterIndexerAddr, id.String(), readCh, writeCh)
	interval := time.NewTicker(10 * time.Second)
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
				break
			}
			filteredCoins, _, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
			if err != nil {
				panic(err)
			}
			updateState(filteredCoins, lastPRVIndex, lastTokenIndex)
		}
		log.Println("finish scanning coins in", time.Since(startTime))
		assignedOTAKeys.Unlock()
	}
}

func cleanAssignedOTA() {
	assignedOTAKeys.Lock()
	assignedOTAKeys.Keys = make(map[int][]*OTAkeyInfo)
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
	err := updateSubmittedOTAKey()
	if err != nil {
		panic(err)
	}
	if len(coinsToUpdate) > 0 {
		log.Println("\n=========================================")
		log.Println("len(coinsToUpdate)", len(coinsToUpdate))
		log.Println("=========================================\n")
		err := database.DBUpdateCoins(coinsToUpdate)
		if err != nil {
			panic(err)
		}
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
}

func filterCoinsByOTAKey(coinList []shared.CoinData) (map[string][]shared.CoinData, []shared.CoinData, map[int]uint64, map[int]uint64, error) {
	lastPRVIndex := make(map[int]uint64)
	lastTokenIndex := make(map[int]uint64)
	tokenListLock.RLock()
	if len(assignedOTAKeys.Keys) > 0 {
		otaCoins := make(map[string][]shared.CoinData)
		var otherCoins []shared.CoinData
		startTime := time.Now()
		var wg sync.WaitGroup
		tempOTACoinsCh := make(chan map[string]shared.CoinData, shared.ServiceCfg.MaxConcurrentOTACheck)
		for idx, c := range coinList {
			log.Println("coinIdx", c.CoinIndex)
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
					pass, tokenID, _ = doesCoinBelongToKeySet(newCoin, keyData.keyset, lastTokenIDMap)
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
		tokenListLock.RUnlock()
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

	log.Println("minPRVIdx", minPRVIdx, minTokenIdx)
	return minPRVIdx, minTokenIdx
}

func GetUnknownCoinsFromDB(fromPRVIndex, fromTokenIndex map[int]uint64) []shared.CoinData {
	var result []shared.CoinData
	for shardID, v := range fromPRVIndex {
		if v != 0 {
			v += 1
		}
		coinList, err := database.DBGetUnknownCoinsV2(shardID, common.PRVCoinID.String(), int64(v), 1000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	for shardID, v := range fromTokenIndex {
		if v != 0 {
			v += 1
		}
		coinList, err := database.DBGetUnknownCoinsV2(shardID, common.ConfidentialAssetID.String(), int64(v), 1000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	return result
}
