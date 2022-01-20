package otaindexer

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	"github.com/kamva/mgm/v3/operator"
	uuid "github.com/satori/go.uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
)

var assignedOTAKeys = struct {
	sync.RWMutex
	Keys      map[int][]*OTAkeyInfo
	TotalKeys int
}{}
var workerID string
var willRun bool

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
		case RUN:
			willRun = true
		case REINDEX:
			pubkey, _, err := base58.Base58Check{}.Decode(keyAction.Key.Pubkey)
			if err != nil {
				log.Fatalln(err)
			}
			shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
			for _, v := range assignedOTAKeys.Keys[int(shardID)] {
				if v.OTAKey == keyAction.Key.OTAKey {
					for tokenID, coinInfo := range keyAction.Key.KeyInfo.CoinIndex {
						v.KeyInfo.CoinIndex[tokenID] = coinInfo
						v.KeyInfo.TotalReceiveTxs[tokenID] = keyAction.Key.KeyInfo.TotalReceiveTxs[tokenID]
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
				KeyInfo: keyAction.Key.KeyInfo,
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
			log.Printf("success added key %v for indexing", keyAction.Key.Pubkey)
		}
		assignedOTAKeys.Unlock()
	}
}

func StartOTAIndexing() {
	log.Println("initiating ota-indexing-service...")
	id := uuid.NewV4()
	workerID = id.String()
	readCh := make(chan []byte)
	writeCh := make(chan []byte)
	go connectMasterIndexer(shared.ServiceCfg.MasterIndexerAddr, id.String(), readCh, writeCh)
	go processMsgFromMaster(readCh, writeCh)
	time.Sleep(10 * time.Second)
	for {
		time.Sleep(5 * time.Second)
		err := retrieveTokenIDList()
		if err != nil {
			panic(err)
		}
		if willRun {
			log.Println("scanning coins...")
			if len(assignedOTAKeys.Keys) == 0 {
				log.Println("len(assignedOTAKeys.Keys) == 0")
				continue
			}
			assignedOTAKeys.Lock()
			scanOTACoins()
			scanTxsSwap()
			assignedOTAKeys.Unlock()
		}
	}
}

func cleanAssignedOTA() {
	assignedOTAKeys.Lock()
	assignedOTAKeys.Keys = make(map[int][]*OTAkeyInfo)
	assignedOTAKeys.TotalKeys = 0
	assignedOTAKeys.Unlock()
}

func scanOTACoins() {
	startTime := time.Now()
	shardKeyGroup := make(map[int]map[string]map[uint64][]*OTAkeyInfo)
	for shardID, keyinfos := range assignedOTAKeys.Keys {
		coinIndexs := groupLastScannedIndexs(keyinfos)
		keysGroup := groupKeysToCoinIndex(coinIndexs, keyinfos)
		shardKeyGroup[shardID] = keysGroup
	}

	coinsToUpdate := []shared.CoinData{}
	txsToUpdate := make(map[string]map[string][]string)

	tokenListLock.RLock()
	tokenIDMap := make(map[string]string)
	for k, v := range lastTokenIDMap {
		tokenIDMap[k] = v
	}
	nftIDMap := make(map[string]string)
	for k, v := range lastNFTIDMap {
		nftIDMap[k] = v
	}
	tokenListLock.RUnlock()

	var wg sync.WaitGroup
	for shardID, tkMap := range shardKeyGroup {
		wg.Add(1)
		go func(tkIndexMap map[string]map[uint64][]*OTAkeyInfo, sID int) {
			for tk, indices := range tkIndexMap {
				for index, keys := range indices {
					coinList := GetUnknownCoinsV2(sID, index, tk)
					var newTokenIDMap map[string]string
					var newNFTIDMap map[string]string

					if tk == common.ConfidentialAssetID.String() {
						newTokenIDMap = tokenIDMap
						newNFTIDMap = nftIDMap
					}
					if len(coinList) == 0 {
						continue
					}
					indexedCoins, txsUpdate := filterCoinsByOTAKeyV2(coinList, keys, newTokenIDMap, newNFTIDMap)
					coinsToUpdate = append(coinsToUpdate, indexedCoins...)
					for pubkey, tokenTxs := range txsUpdate {
						for tokenID, txs := range tokenTxs {
							if _, ok := txsToUpdate[pubkey]; ok {
								txsToUpdate[pubkey][tokenID] = txs
							} else {
								txsToUpdate[pubkey] = make(map[string][]string)
								txsToUpdate[pubkey][tokenID] = txs
							}
						}
					}
				}
			}
			wg.Done()
		}(tkMap, shardID)
	}
	wg.Wait()

	if len(coinsToUpdate) > 0 {
		log.Println("\n=========================================")
		log.Println("len(coinsToUpdate)", len(coinsToUpdate))
		log.Println("=========================================\n")
		err := database.DBUpdateCoins(coinsToUpdate, context.Background())
		if err != nil {
			panic(err)
		}
	}

	if len(txsToUpdate) > 0 {
		for pubkey, tokenTxs := range txsToUpdate {
			for tokenID, txHashs := range tokenTxs {
				err := database.DBUpdateTxPubkeyReceiverAndTokenID(txHashs, pubkey, tokenID, context.Background())
				if err != nil {
					panic(err)
				}
			}
		}
	}

	err = updateSubmittedOTAKey(context.Background())
	if err != nil {
		panic(err)
	}

	//Old
	// lastPRVIndex, lastTokenIndex := GetOTAKeyListMinScannedCoinIndex(assignedOTAKeys.Keys)
	// //scan coins
	// for {
	// 	coinList := GetUnknownCoinsFromDB(lastPRVIndex, lastTokenIndex)
	// 	if len(coinList) == 0 {
	// 		break
	// 	}
	// 	filteredCoins := make(map[string][]shared.CoinData)
	// 	filteredCoins, _, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	updateCoinState(filteredCoins, lastPRVIndex, lastTokenIndex)
	// }
	log.Printf("worker/%v finish scanning coins in %v\n", workerID, time.Since(startTime))
}

func scanTxsSwap() {
	startTime := time.Now()
	lastTxIndex := GetOTAKeyListMinScannedTxIndex()
	//scan coins
	for {
		txList, err := getTxToProcess(lastTxIndex, 5000)
		if err != nil {
			log.Println(err)
			continue
		}
		if len(txList) == 0 {
			break
		}
		filteredTx := []shared.TxData{}
		filteredTx, lastTxIndex, err = filterTxsByOTAKey(txList)
		if err != nil {
			panic(err)
		}
		updateTxState(filteredTx, lastTxIndex)
	}
	log.Printf("worker/%v finish scanning coins in %v\n", workerID, time.Since(startTime))
}

func updateTxState(txList []shared.TxData, lastTxIndex map[int]string) {
	for shardID, keyDatas := range assignedOTAKeys.Keys {
		for _, keyData := range keyDatas {
			keyData.KeyInfo.LastScanTxID = lastTxIndex[shardID]
		}
	}
	err := database.DBUpdateTxsWithPubkeyReceiver(txList)
	if err != nil {
		panic(err)
	}
}

func updateCoinState(otaCoinList map[string][]shared.CoinData, lastPRVIndex, lastTokenIndex map[int]uint64) {
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
			if len(keyData.KeyInfo.NFTIndex) == 0 {
				keyData.KeyInfo.NFTIndex = make(map[string]shared.CoinInfo)
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
					if !v.IsNFT {
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
					} else {
						if _, ok := keyData.KeyInfo.NFTIndex[v.RealTokenID]; !ok {
							keyData.KeyInfo.NFTIndex[v.RealTokenID] = shared.CoinInfo{
								Start: v.CoinIndex,
								End:   v.CoinIndex,
								Total: 1,
							}
						} else {
							d := keyData.KeyInfo.NFTIndex[v.RealTokenID]
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
							keyData.KeyInfo.NFTIndex[v.RealTokenID] = d
						}
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

	if len(coinsToUpdate) > 0 {
		log.Println("\n=========================================")
		log.Println("len(coinsToUpdate)", len(coinsToUpdate))
		log.Println("=========================================\n")
		err := database.DBUpdateCoins(coinsToUpdate, context.Background())
		if err != nil {
			panic(err)
		}
	}
	err := updateSubmittedOTAKey(context.Background())
	if err != nil {
		panic(err)
	}

	for key, tokenTxs := range totalTxs {
		for tokenID, txList := range tokenTxs {
			txHashs := []string{}
			for txHash := range txList {
				txHashs = append(txHashs, txHash)
			}
			err := database.DBUpdateTxPubkeyReceiverAndTokenID(txHashs, pubkeys[key], tokenID, context.Background())
			if err != nil {
				panic(err)
			}
		}
	}
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
	nftIDMap := make(map[string]string)
	for k, v := range lastNFTIDMap {
		nftIDMap[k] = v
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
				isNFT := false
				for _, keyData := range assignedOTAKeys.Keys[cn.ShardID] {
					if _, ok := keyData.KeyInfo.CoinIndex[cn.TokenID]; ok {
						if cn.CoinIndex < keyData.KeyInfo.CoinIndex[cn.TokenID].LastScanned {
							continue
						}
					}
					checkToken := true
					if cn.IsNFT {
						checkToken = false
					}
					pass, tokenID, _, isNFT = doesCoinBelongToKeySet(newCoin, keyData.keyset, tokenIDMap, nftIDMap, checkToken)
					if pass {
						if !cn.IsNFT {
							cn.RealTokenID = tokenID
						}
						if isNFT {
							cn.IsNFT = isNFT
						}
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

func updateSubmittedOTAKey(ctx context.Context) error {
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
		err := database.DBUpdateKeyInfoV2(doc, KeyInfoList[idx], ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetOTAKeyListMinScannedCoinIndex(keylist map[int][]*OTAkeyInfo) (map[int]uint64, map[int]uint64) {
	minPRVIdx := make(map[int]uint64)
	minTokenIdx := make(map[int]uint64)

	for shardID, keys := range keylist {
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
		coinList, err := database.DBGetUnknownCoinsV21(shardID, common.PRVCoinID.String(), int64(v), 10000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	for shardID, v := range fromTokenIndex {
		if v != 0 {
			v += 1
		}
		coinList, err := database.DBGetUnknownCoinsV21(shardID, common.ConfidentialAssetID.String(), int64(v), 10000)
		if err != nil {
			panic(err)
		}
		result = append(result, coinList...)
	}
	return result
}

func GetUnknownCoinsV2(shardID int, fromIndex uint64, tokenID string) []shared.CoinData {
	var result []shared.CoinData
	coinList, err := database.DBGetUnknownCoinsV21(shardID, common.PRVCoinID.String(), int64(fromIndex), 10000)
	if err != nil {
		panic(err)
	}
	result = append(result, coinList...)
	return result
}

func getTxToProcess(lastID map[int]string, limit int64) ([]shared.TxData, error) {
	result := []shared.TxData{}
	for shardID, v := range lastID {
		var txList []shared.TxData
		metas := []string{strconv.Itoa(metadataCommon.Pdexv3TradeRequestMeta)}
		var obID primitive.ObjectID
		var err error
		if v == "" {
			obID = primitive.ObjectID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
		} else {
			obID, err = primitive.ObjectIDFromHex(v)
			if err != nil {
				return nil, err
			}
		}

		filter := bson.M{
			"_id":      bson.M{operator.Gt: obID},
			"shardid":  bson.M{operator.Eq: shardID},
			"metatype": bson.M{operator.In: metas},
		}
		err = mgm.Coll(&shared.TxData{}).SimpleFind(&txList, filter, &options.FindOptions{
			Sort:  bson.D{{"locktime", 1}},
			Limit: &limit,
		})
		if err != nil {
			return nil, err
		}
		for _, v := range txList {
			if len(v.PubKeyReceivers) == 0 {
				result = append(result, v)
			}
		}

	}

	return result, nil
}

func GetOTAKeyListMinScannedTxIndex() map[int]string {
	minTxIdx := make(map[int]string)

	for shardID, keys := range assignedOTAKeys.Keys {
		minTxIdx[shardID] = keys[0].KeyInfo.LastScanTxID
		for _, keyData := range keys {
			if strings.Compare(keyData.KeyInfo.LastScanTxID, minTxIdx[shardID]) == -1 {
				minTxIdx[shardID] = keyData.KeyInfo.LastScanTxID
			}
			if keyData.KeyInfo.LastScanTxID == "" {
				id := primitive.ObjectID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				minTxIdx[shardID] = id.Hex()
				break
			}
		}
	}

	log.Printf("worker/%v minTxIdx %v \n", workerID, minTxIdx)
	return minTxIdx
}

func filterTxsByOTAKey(txList []shared.TxData) ([]shared.TxData, map[int]string, error) {

	lastTxID := make(map[int]string)
	if len(assignedOTAKeys.Keys) > 0 {
		var otaTxs []shared.TxData
		// var otherTxs []shared.TxData
		startTime := time.Now()
		var wg sync.WaitGroup
		tempTxsCh := make(chan map[string]shared.TxData, shared.ServiceCfg.MaxConcurrentOTACheck)
		for idx, tx := range txList {
			wg.Add(1)
			go func(txd shared.TxData) {
				pass := false
				for _, keyData := range assignedOTAKeys.Keys[tx.ShardID] {
					if strings.Compare(txd.TxHash, keyData.KeyInfo.LastScanTxID) != -1 {
						continue
					}
					txRan, pubkey, err := extractPubkeyAndTxRandom(txd)
					if err != nil {
						panic(err)
					}

					pass = checkPubkeyAndTxRandom(*txRan, *pubkey, keyData.keyset)
					if pass {
						tempTxsCh <- map[string]shared.TxData{keyData.Pubkey: txd}
						break
					}
				}
				if !pass {
					tempTxsCh <- map[string]shared.TxData{"nil": txd}
				}
				wg.Done()
			}(tx)
			if (idx+1)%shared.ServiceCfg.MaxConcurrentOTACheck == 0 || idx+1 == len(txList) {
				wg.Wait()
				close(tempTxsCh)
				for k := range tempTxsCh {
					for key, v := range k {
						if key == "nil" {
							// otherTxs = append(otherTxs, v)
							continue
						}
						v.PubKeyReceivers = append(v.PubKeyReceivers, key)
						otaTxs = append(otaTxs, v)
					}
				}
				// if idx+1 != len(coinList) {
				tempTxsCh = make(chan map[string]shared.TxData, shared.ServiceCfg.MaxConcurrentOTACheck)
				// }
			}
			if _, ok := lastTxID[tx.ShardID]; !ok {
				lastTxID[tx.ShardID] = tx.TxHash
			} else {
				if strings.Compare(tx.TxHash, lastTxID[tx.ShardID]) == -1 {
					lastTxID[tx.ShardID] = tx.TxHash
				}
			}
		}
		close(tempTxsCh)
		log.Printf("filtered %v coins with %v keys in %v", len(txList), assignedOTAKeys.TotalKeys, time.Since(startTime))
		return otaTxs, lastTxID, errors.New("no key to scan")
	}
	return nil, nil, errors.New("no key to scan")
}

func groupLastScannedIndexs(keys []*OTAkeyInfo) map[string][]uint64 {
	tempIndexMap := make(map[string][]uint64)
	indexMap := make(map[string][]uint64)
	for _, v := range keys {
		prvLsc := uint64(0)
		if _, ok := v.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
			prvLsc = v.KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned
		}
		tokenLsc := uint64(0)
		if _, ok := v.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
			tokenLsc = v.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
		}
		tempIndexMap[common.PRVCoinID.String()] = append(tempIndexMap[common.PRVCoinID.String()], prvLsc)
		tempIndexMap[common.ConfidentialAssetID.String()] = append(tempIndexMap[common.ConfidentialAssetID.String()], tokenLsc)
	}
	tempIndexMap[common.PRVCoinID.String()] = removeDuplicateInt(tempIndexMap[common.PRVCoinID.String()])
	tempIndexMap[common.ConfidentialAssetID.String()] = removeDuplicateInt(tempIndexMap[common.ConfidentialAssetID.String()])

	sort.SliceStable(tempIndexMap[common.PRVCoinID.String()], func(i, j int) bool {
		return tempIndexMap[common.PRVCoinID.String()][i] < tempIndexMap[common.PRVCoinID.String()][j]
	})
	sort.SliceStable(tempIndexMap[common.ConfidentialAssetID.String()], func(i, j int) bool {
		return tempIndexMap[common.ConfidentialAssetID.String()][i] < tempIndexMap[common.ConfidentialAssetID.String()][j]
	})

	indexMap[common.ConfidentialAssetID.String()] = reduceSlice(tempIndexMap[common.ConfidentialAssetID.String()], 10000)
	indexMap[common.PRVCoinID.String()] = reduceSlice(tempIndexMap[common.PRVCoinID.String()], 10000)

	return indexMap
}

func removeDuplicateInt(slice []uint64) []uint64 {
	allKeys := make(map[uint64]bool)
	list := []uint64{}
	for _, item := range slice {
		if _, value := allKeys[item]; !value {
			allKeys[item] = true
			list = append(list, item)
		}
	}
	return list
}

func reduceSlice(slice []uint64, r uint64) []uint64 {
	var result []uint64
	i := uint64(0)
	for idx, v := range slice {
		if i+r < v || v == 0 || idx == len(slice)-1 {
			result = append(result, v)
			i = v + r
		}
	}
	return result
}

func groupKeysToCoinIndex(coinIndex map[string][]uint64, keys []*OTAkeyInfo) map[string]map[uint64][]*OTAkeyInfo {
	result := make(map[string]map[uint64][]*OTAkeyInfo)
	result[common.ConfidentialAssetID.String()] = make(map[uint64][]*OTAkeyInfo)
	result[common.PRVCoinID.String()] = make(map[uint64][]*OTAkeyInfo)

	for token, idxs := range coinIndex {
		for _, kinfo := range keys {
			if cinfo, ok := kinfo.KeyInfo.CoinIndex[token]; ok {
				for _, v := range idxs {
					if cinfo.LastScanned <= v {
						result[token][v] = append(result[token][v], kinfo)
						break
					}
				}
			} else {
				result[token][0] = append(result[token][0], kinfo)
			}
		}
	}
	return result
}
