package otaindexer

import (
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/shared"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"go.mongodb.org/mongo-driver/bson"
)

//manage submitted otakey

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys      map[int][]*OTAkeyInfo
	TotalKeys int
}{}

func loadSubmittedOTAKey() {
	keys, err := database.DBGetSubmittedOTAKeys(shared.ServiceCfg.IndexerBucketID, 0)
	if err != nil {
		log.Fatalln(err)
	}
	Submitted_OTAKey.Keys = make(map[int][]*OTAkeyInfo)
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
		Submitted_OTAKey.Keys[int(shardID)] = append(Submitted_OTAKey.Keys[int(shardID)], &k)
		Submitted_OTAKey.TotalKeys += 1
	}
	Submitted_OTAKey.Unlock()
	log.Printf("Loaded %v keys\n", len(keys))
}

func updateSubmittedOTAKey() error {
	Submitted_OTAKey.Lock()
	docs := []interface{}{}
	KeyInfoList := []*shared.KeyInfoData{}
	for _, keys := range Submitted_OTAKey.Keys {
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
			Submitted_OTAKey.Unlock()
			return err
		}
	}
	Submitted_OTAKey.Unlock()
	return nil
}

func InitOTAIndexingService() {
	log.Println("initiating ota-indexing-service...")
	common.MaxShardNumber = shared.ServiceCfg.NumOfShard
	loadSubmittedOTAKey()
	interval := time.NewTicker(10 * time.Second)
	var coinList []shared.CoinData
	// var lastScannedKeys int
	updateState := func(otaCoinList map[string][]shared.CoinData, lastPRVIndex, lastTokenIndex map[int]uint64) {
		pubkeys := make(map[string]string)
		Submitted_OTAKey.Lock()
		for shardID, keyDatas := range Submitted_OTAKey.Keys {
			for _, keyData := range keyDatas {
				pubkeys[keyData.OTAKey] = keyData.Pubkey
				if len(keyData.KeyInfo.CoinIndex) == 0 {
					keyData.KeyInfo.CoinIndex = make(map[string]shared.CoinInfo)
				}
				if cd, ok := otaCoinList[keyData.OTAKey]; ok {
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
		Submitted_OTAKey.Unlock()
		err := updateSubmittedOTAKey()
		if err != nil {
			panic(err)
		}
		coinsToUpdate := []shared.CoinData{}

		txToUpdate := make(map[string][]string)
		for key, coins := range otaCoinList {
			for _, coin := range coins {
				coin.OTASecret = key
				coinsToUpdate = append(coinsToUpdate, coin)
				txToUpdate[key] = append(txToUpdate[key], coin.TxHash)
			}

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

		for key, txHashs := range txToUpdate {
			err := database.DBUpdateTxPubkeyReceiver(txHashs, pubkeys[key])
			if err != nil {
				panic(err)
			}
		}
	}

	for {
		<-interval.C
		err := retrieveTokenIDList()
		if err != nil {
			panic(err)
		}
		keys, err := database.DBGetSubmittedOTAKeys(shared.ServiceCfg.IndexerBucketID, int64(Submitted_OTAKey.TotalKeys))
		if err != nil {
			log.Fatalln(err)
		}
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
			Submitted_OTAKey.Keys[int(shardID)] = append(Submitted_OTAKey.Keys[int(shardID)], &k)
			Submitted_OTAKey.TotalKeys += 1
		}
		Submitted_OTAKey.Unlock()

		log.Println("scanning coins...")
		if len(Submitted_OTAKey.Keys) == 0 {
			log.Println("len(Submitted_OTAKey.Keys) == 0")
			continue
		}
		startTime := time.Now()
		minPRVIndex, minTokenIndex := GetOTAKeyListMinScannedCoinIndex()

		// if time.Since(coinCache.Time) < 10*time.Second {
		// 	log.Println("getting coins from cache...")
		// 	coinList, _, _ = coinCache.Read()
		// 	if len(coinList) == 0 {
		// 		log.Println("len(coinList) == 0")
		// 		continue
		// 	}
		// 	if Submitted_OTAKey.TotalKeys != lastScannedKeys {
		// 		filteredCoins, remainingCoins, lastPRVIndex, lastTokenIndex, err := filterCoinsByOTAKey(coinList)
		// 		if err != nil {
		// 			panic(err)
		// 		}
		// 		updateState(filteredCoins, lastPRVIndex, lastTokenIndex)
		// 		coinCache.Update(remainingCoins, lastPRVIndex, lastTokenIndex)
		// 		lastScannedKeys = Submitted_OTAKey.TotalKeys
		// 	}
		// 	continue
		// }

		coinList = GetUnknownCoinsFromDB(minPRVIndex, minTokenIndex)
		if len(coinList) == 0 {
			log.Println("len(coinList) == 0")
			continue
		}
		remainingCoins := []shared.CoinData{}
		filteredCoins, remaining, lastPRVIndex, lastTokenIndex, err := filterCoinsByOTAKey(coinList)
		if err != nil {
			panic(err)
		}
		updateState(filteredCoins, lastPRVIndex, lastTokenIndex)
		remainingCoins = append(remainingCoins, remaining...)
		// coinCache.Update(remainingCoins, lastPRVIndex, lastTokenIndex)
		for {
			coinList = GetUnknownCoinsFromDB(lastPRVIndex, lastTokenIndex)
			if len(coinList) == 0 {
				break
			}
			filteredCoins, remainingCoins, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
			if err != nil {
				panic(err)
			}
			// lastScannedKeys = Submitted_OTAKey.TotalKeys
			updateState(filteredCoins, lastPRVIndex, lastTokenIndex)
			remainingCoins = append(remainingCoins, remaining...)
			// coinCache.Update(remainingCoins, lastPRVIndex, lastTokenIndex)
		}
		_ = remainingCoins
		log.Println("finish scanning coins in", time.Since(startTime))
	}
}

func filterCoinsByOTAKey(coinList []shared.CoinData) (map[string][]shared.CoinData, []shared.CoinData, map[int]uint64, map[int]uint64, error) {
	lastPRVIndex := make(map[int]uint64)
	lastTokenIndex := make(map[int]uint64)
	tokenListLock.RLock()
	if len(Submitted_OTAKey.Keys) > 0 {
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
				for _, keyData := range Submitted_OTAKey.Keys[cn.ShardID] {
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
		log.Printf("filtered %v coins with %v keys in %v", len(coinList), Submitted_OTAKey.TotalKeys, time.Since(startTime))
		log.Println("lastPRVIndex", lastPRVIndex, lastTokenIndex)
		tokenListLock.RUnlock()
		return otaCoins, otherCoins, lastPRVIndex, lastTokenIndex, nil
	}
	return nil, nil, nil, nil, errors.New("no key to scan")
}

func GetOTAKeyListMinScannedCoinIndex() (map[int]uint64, map[int]uint64) {
	Submitted_OTAKey.RLock()
	minPRVIdx := make(map[int]uint64)
	minTokenIdx := make(map[int]uint64)
	for shardID, keys := range Submitted_OTAKey.Keys {
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
	Submitted_OTAKey.RUnlock()
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

var OTAAssignChn chan OTAAssignRequest
var maxBucketSize uint64

func StartBucketAssigner(maxSize uint64, maxBucketNum int) {
	maxBucketSize = maxSize
	OTAAssignChn = make(chan OTAAssignRequest)
	bucketInfo, err := database.DBGetBucketStats(maxBucketNum)
	if err != nil {
		panic(err)
	}
	for {
		request := <-OTAAssignChn
		leastOccupiedBucketID := 0
		for bucketID, keyLength := range bucketInfo {
			if keyLength < bucketInfo[leastOccupiedBucketID] {
				leastOccupiedBucketID = bucketID
			}
		}
		request.Key.BucketID = leastOccupiedBucketID
		err := database.DBSaveSubmittedOTAKeys([]shared.SubmittedOTAKeyData{*request.Key})
		if err != nil {
			go func() {
				request.Respond <- err
			}()
		} else {
			bucketInfo[leastOccupiedBucketID] += 1
			request.Respond <- nil
			if bucketInfo[leastOccupiedBucketID] > maxBucketSize {
				log.Printf("Bucket %v has more than %v keys", leastOccupiedBucketID, maxBucketSize)
			}
		}
	}
}
