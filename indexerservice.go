package main

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
)

//manage submitted otakey
type OTAkeyInfo struct {
	ShardID int
	Pubkey  string
	OTAKey  string
	keyset  *incognitokey.KeySet
	KeyInfo *KeyInfoData
}

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys []*OTAkeyInfo
}{}

func loadSubmittedOTAKey() {
	keys, err := DBGetOTAKeys(serviceCfg.IndexerBucketID, 0)
	if err != nil {
		log.Fatalln(err)
	}
	Submitted_OTAKey.Lock()
	for _, key := range keys {
		pubkey, _, _ := base58.Base58Check{}.Decode(key.Pubkey)
		keyBytes, _, _ := base58.Base58Check{}.Decode(key.OTAKey)
		keyBytes = append(keyBytes, pubkey...)
		if len(keyBytes) != 64 {
			log.Fatalln(errors.New("keyBytes length isn't 64"))
		}
		if len(keyBytes) != 64 {
			log.Fatalln(errors.New("keyBytes length isn't 64"))
		}
		otaKey := OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
		data, err := DBGetCoinV2PubkeyInfo(key.OTAKey)
		if err != nil {
			log.Fatalln(err)
		}
		k := OTAkeyInfo{
			KeyInfo: data,
			ShardID: int(shardID),
			OTAKey:  key.OTAKey,
			Pubkey:  key.Pubkey,
			keyset:  ks,
		}
		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &k)
	}
	Submitted_OTAKey.Unlock()

	// pipeline := bson.A{}
	// changeStream, err := mgm.Coll(&SubmittedOTAKeyData{}).Watch(context.Background(), pipeline)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// for changeStream.Next(context.Background()) {
	// 	key := SubmittedOTAKeyData{}
	// 	if err := changeStream.Decode(key); err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	if key.BucketID == serviceCfg.IndexerBucketID {
	// 		pubkey, _, _ := base58.Base58Check{}.Decode(key.Pubkey)
	// 		keyBytes, _, _ := base58.Base58Check{}.Decode(key.OTAKey)
	// 		keyBytes = append(keyBytes, pubkey...)
	// 		if len(keyBytes) != 64 {
	// 			log.Fatalln(errors.New("keyBytes length isn't 64"))
	// 		}
	// 		if len(keyBytes) != 64 {
	// 			log.Fatalln(errors.New("keyBytes length isn't 64"))
	// 		}
	// 		otaKey := OTAKeyFromRaw(keyBytes)
	// 		ks := &incognitokey.KeySet{}
	// 		ks.OTAKey = otaKey
	// 		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
	// 		data, err := DBGetCoinV2PubkeyInfo(key.OTAKey)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		k := OTAkeyInfo{
	// 			KeyInfo: data,
	// 			ShardID: int(shardID),
	// 			OTAKey:  key.OTAKey,
	// 			Pubkey:  key.Pubkey,
	// 			keyset:  ks,
	// 		}

	// 		Submitted_OTAKey.Lock()
	// 		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &k)
	// 		Submitted_OTAKey.Unlock()
	// 	}
	// }

	// if err := changeStream.Close(context.Background()); err != nil {
	// 	log.Fatal(err)
	// }

	interval := time.NewTicker(5 * time.Second)
	for {
		<-interval.C
		keys, err := DBGetOTAKeys(serviceCfg.IndexerBucketID, int64(len(Submitted_OTAKey.Keys)))
		if err != nil {
			log.Fatalln(err)
		}
		Submitted_OTAKey.Lock()
		for _, key := range keys {
			pubkey, _, _ := base58.Base58Check{}.Decode(key.Pubkey)
			keyBytes, _, _ := base58.Base58Check{}.Decode(key.OTAKey)
			keyBytes = append(keyBytes, pubkey...)
			if len(keyBytes) != 64 {
				log.Fatalln(errors.New("keyBytes length isn't 64"))
			}
			if len(keyBytes) != 64 {
				log.Fatalln(errors.New("keyBytes length isn't 64"))
			}
			otaKey := OTAKeyFromRaw(keyBytes)
			ks := &incognitokey.KeySet{}
			ks.OTAKey = otaKey
			shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
			data, err := DBGetCoinV2PubkeyInfo(key.OTAKey)
			if err != nil {
				log.Fatalln(err)
			}
			k := OTAkeyInfo{
				KeyInfo: data,
				ShardID: int(shardID),
				OTAKey:  key.OTAKey,
				Pubkey:  key.Pubkey,
				keyset:  ks,
			}
			Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &k)
		}
		Submitted_OTAKey.Unlock()

	}
}

func updateSubmittedOTAKey() error {
	Submitted_OTAKey.Lock()
	docs := []interface{}{}
	for _, key := range Submitted_OTAKey.Keys {
		update := bson.M{
			"$set": key.KeyInfo,
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
		_, err := mgm.Coll(&KeyInfoDataV2{}).UpdateByID(ctx, Submitted_OTAKey.Keys[idx].KeyInfo.GetID(), doc)
		if err != nil {
			return err
		}
	}
	Submitted_OTAKey.Unlock()
	return nil
}

func initOTAIndexingService() {
	log.Println("initiating ota-indexing-service...")

	go loadSubmittedOTAKey()
	// wl, _ := wallet.Base58CheckDeserialize("112t8rnXoBXrThDTACHx2rbEq7nBgrzcZhVZV4fvNEcGJetQ13spZRMuW5ncvsKA1KvtkauZuK2jV8pxEZLpiuHtKX3FkKv2uC5ZeRC8L6we")
	// key := wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rnbcZ92v5omVfbXf1gu7j7S1xxr2eppxitbHfjAMHWdLLBjBcQSv1X1cKjarJLffrPGwBhqZzBvEeA9PhtKeM8ALWiWjhUzN5Fi6WVC")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rnZUQXxcbayAZvyyZyKDhwVJBLkHuTKMhrS51nQZcXKYXGopUTj22JtZ8KxYQcak54KUQLhimv1GLLPFk1cc8JCHZ2JwxCRXGsg4gXU")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rnXDS4cAjFVgCDEw4sWGdaqQSbKLRH1Hu4nUPBFPJdn29YgUei2KXNEtC8mhi1sEZb1V3gnXdAXjmCuxPa49rbHcH9uNaf85cnF3tMw")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rnYoioTRNsM8gnUYt54ThWWrRnG4e1nRX147MWGbEazYP7RWrEUB58JLnBjKhh49FMS5o5ttypZucfw5dFYMAsgDUsHPa9BAasY8U1i")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8roafGgHL1rhAP9632Yef3sx5k8xgp8cwK4MCJsCL1UWcxXvpzg97N4dwvcD735iKf31Q2ZgrAvKfVjeSUEvnzKJyyJD3GqqSZdxN4or")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rnZDRztVgPjbYQiXS7mJgaTzn66NvHD7Vus2SrhSAY611AzADsPFzKjKQCKWTgbkgYrCPo9atvSMoCf9KT23Sc7Js9RKhzbNJkxpJU6")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// wl, _ = wallet.Base58CheckDeserialize("112t8roafGgHL1rhAP9632Yef3sx5k8xgp8cwK4MCJsCL1UWcxXvpzg97N4dwvcD735iKf31Q2ZgrAvKfVjeSUEvnzKJyyJD3GqqSZdxN4or")
	// key = wl.Base58CheckSerialize(wallet.OTAKeyType)
	// addOTAKey(key, 0, 0, 0)

	// panic(9)
	interval := time.NewTicker(10 * time.Second)
	var coinList, coinsToUpdate []CoinData
	var lastPRVIndex, lastTokenIndex uint64

	updateState := func(coins map[string][]CoinData) {
		Submitted_OTAKey.Lock()
		for _, keyData := range Submitted_OTAKey.Keys {
			if len(keyData.KeyInfo.CoinIndex) == 0 {
				keyData.KeyInfo.CoinIndex = make(map[string]CoinInfo)
			}
			if cd, ok := coins[keyData.OTAKey]; ok {
				sort.Slice(cd, func(i, j int) bool { return cd[i].CoinIndex < cd[j].CoinIndex })
				for _, v := range cd {
					if _, ok := keyData.KeyInfo.CoinIndex[v.TokenID]; !ok {
						keyData.KeyInfo.CoinIndex[v.TokenID] = CoinInfo{}
					}
					if keyData.KeyInfo.CoinIndex[v.TokenID].Total == 0 {
						d := keyData.KeyInfo.CoinIndex[v.TokenID]
						d.Start = v.CoinIndex
						d.End = v.CoinIndex
						d.Total = 1
						keyData.KeyInfo.CoinIndex[v.TokenID] = d
					} else {
						d := keyData.KeyInfo.CoinIndex[v.TokenID]
						d.End = v.CoinIndex
						d.Total += 1
						keyData.KeyInfo.CoinIndex[v.TokenID] = d
					}
				}
			}

			if d, ok := keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
				if d.LastScanned < lastPRVIndex {
					d.LastScanned = lastPRVIndex
					keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()] = d
				}
			} else {
				d.LastScanned = lastPRVIndex
				keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()] = d
			}

			if d, ok := keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
				if d.LastScanned < lastTokenIndex {
					d.LastScanned = lastTokenIndex
					keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = d
				}
			} else {
				d.LastScanned = lastTokenIndex
				keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = d
			}
		}
		Submitted_OTAKey.Unlock()

		err := updateSubmittedOTAKey()
		if err != nil {
			panic(err)
		}
		if len(coinsToUpdate) > 0 {
			log.Println("\n=========================================")
			log.Println("len(coinsToUpdate)", len(coinsToUpdate))
			log.Println("=========================================\n")
			err := DBUpdateCoins(coinsToUpdate)
			if err != nil {
				panic(err)
			}
		}
	}

	// DBGetOTAKeys(serviceCfg.IndexerBucketID)
	for {
		<-interval.C
		log.Println("scanning coins...")
		if len(Submitted_OTAKey.Keys) == 0 {
			log.Println("len(Submitted_OTAKey.Keys) == 0")
			continue
		}
		startTime := time.Now()

		minPRVIndex, minTokenIndex := GetOTAKeyListMinScannedCoinIndex()

		if time.Since(coinCache.Time) < 10*time.Second {
			log.Println("getting coins from cache...")
			var cachePRVIndex, cacheTokenIndex uint64
			coinList, cachePRVIndex, cacheTokenIndex = coinCache.Read()
			if len(coinList) == 0 {
				log.Println("len(coinList) == 0")
				continue
			}
			if cachePRVIndex > minPRVIndex || cacheTokenIndex > minTokenIndex {
				filteredCoins, remainingCoins, lastPRVIndex, lastTokenIndex, err := filterCoinsByOTAKey(coinList)
				if err != nil {
					panic(err)
				}
				for key, coins := range filteredCoins {
					for _, coin := range coins {
						coin.OTASecret = key
						coinsToUpdate = append(coinsToUpdate, coin)
					}
				}
				if len(filteredCoins) > 0 {
					coinCache.Update(remainingCoins, lastPRVIndex, lastTokenIndex)
					updateState(filteredCoins)
				}
			}
			continue
		}

		coinList = GetCoinsFromDB(minPRVIndex+1, minTokenIndex+1)
		if len(coinList) == 0 {
			log.Println("len(coinList) == 0")
			continue
		}
		filteredCoins, remainingCoins, lastPRVIndex, lastTokenIndex, err := filterCoinsByOTAKey(coinList)
		if err != nil {
			panic(err)
		}
		for key, coins := range filteredCoins {
			for _, coin := range coins {
				coin.OTASecret = key
				coinsToUpdate = append(coinsToUpdate, coin)
			}
		}
		for {
			coinList = GetCoinsFromDB(lastPRVIndex+1, lastTokenIndex+1)
			if len(coinList) == 0 {
				break
			}
			filteredCoins, remainingCoins, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
			if err != nil {
				panic(err)
			}
			for key, coins := range filteredCoins {
				for _, coin := range coins {
					coin.OTASecret = key
					coinsToUpdate = append(coinsToUpdate, coin)
				}
			}
		}
		coinCache.Update(remainingCoins, lastPRVIndex, lastTokenIndex)
		updateState(filteredCoins)
		log.Println("finish scanning coins in", time.Since(startTime))
	}
}

func filterCoinsByOTAKey(coinList []CoinData) (map[string][]CoinData, []CoinData, uint64, uint64, error) {
	var lastPRVIndex, lastTokenIndex uint64
	if len(Submitted_OTAKey.Keys) > 0 {
		otaCoins := make(map[string][]CoinData)
		var otherCoins []CoinData
		startTime := time.Now()
		Submitted_OTAKey.RLock()
		var wg sync.WaitGroup
		tempOTACoinsCh := make(chan map[string]CoinData, serviceCfg.MaxConcurrentOTACheck)
		for idx, c := range coinList {
			wg.Add(1)
			go func(cn CoinData) {
				newCoin := new(coin.CoinV2)
				err := newCoin.SetBytes(cn.Coin)
				if err != nil {
					panic(err)
				}
				pass := false
				for _, keyData := range Submitted_OTAKey.Keys {
					if _, ok := keyData.KeyInfo.CoinIndex[cn.TokenID]; ok {
						if cn.CoinIndex <= keyData.KeyInfo.CoinIndex[cn.TokenID].LastScanned {
							continue
						}
					}
					pass, _ = newCoin.DoesCoinBelongToKeySet(keyData.keyset)
					if pass {
						tempOTACoinsCh <- map[string]CoinData{keyData.OTAKey: cn}
						break
					}
				}
				if !pass {
					tempOTACoinsCh <- map[string]CoinData{"nil": cn}
				}
				wg.Done()
			}(c)
			if (idx+1)%serviceCfg.MaxConcurrentOTACheck == 0 || idx+1 == len(coinList) {
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
				tempOTACoinsCh = make(chan map[string]CoinData, serviceCfg.MaxConcurrentOTACheck)
			}
			if c.TokenID == common.PRVCoinID.String() {
				if c.CoinIndex > lastPRVIndex {
					lastPRVIndex = c.CoinIndex
				}
			}
			if c.TokenID == common.ConfidentialAssetID.String() {
				if c.CoinIndex > lastTokenIndex {
					lastTokenIndex = c.CoinIndex
				}
			}
		}
		Submitted_OTAKey.RUnlock()
		log.Println("len(otaCoins)", len(otaCoins))
		log.Printf("filtered %v coins with %v keys in %v", len(coinList), len(Submitted_OTAKey.Keys), time.Since(startTime))
		log.Println("lastPRVIndex", lastPRVIndex, lastTokenIndex)
		return otaCoins, otherCoins, lastPRVIndex, lastTokenIndex, nil
	}
	return nil, nil, 0, 0, nil
}

func GetOTAKeyListMinScannedCoinIndex() (uint64, uint64) {
	Submitted_OTAKey.RLock()
	minPRVIdx := uint64(0)
	minTokenIdx := uint64(0)
	if _, ok := Submitted_OTAKey.Keys[0].KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
		minPRVIdx = Submitted_OTAKey.Keys[0].KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned
	}
	if _, ok := Submitted_OTAKey.Keys[0].KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
		minPRVIdx = Submitted_OTAKey.Keys[0].KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
	}
	for _, keyData := range Submitted_OTAKey.Keys {
		if _, ok := keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()]; ok {
			if minPRVIdx > keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned {
				minPRVIdx = keyData.KeyInfo.CoinIndex[common.PRVCoinID.String()].LastScanned
			}
		} else {
			minPRVIdx = 0
		}
		if _, ok := keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]; ok {
			minTokenIdx = keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
			if minTokenIdx > keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned {
				minTokenIdx = keyData.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()].LastScanned
			}
		} else {
			minTokenIdx = 0
		}

	}
	Submitted_OTAKey.RUnlock()
	return minPRVIdx, minTokenIdx
}

func GetCoinsFromDB(fromPRVIndex, fromTokenIndex uint64) []CoinData {
	log.Println("requesting 500 prv from index", fromPRVIndex)
	coinList, err := DBGetCoinsByOTAKey(common.PRVCoinID.String(), "", int64(fromPRVIndex), 500)
	if err != nil {
		panic(err)
	}
	log.Println("requesting 500 token from index", fromTokenIndex)
	coinListTk, err := DBGetCoinsByOTAKey(common.ConfidentialAssetID.String(), "", int64(fromTokenIndex), 500)
	if err != nil {
		panic(err)
	}
	return append(coinList, coinListTk...)
}

type otaAssignRequest struct {
	Key     *SubmittedOTAKeyData
	Respond chan error
}

var otaAssignChn chan otaAssignRequest

func startBucketAssigner() {
	otaAssignChn = make(chan otaAssignRequest)
	bucketInfo, err := DBGetBucketStats(5)
	if err != nil {
		panic(err)
	}
	for {
		request := <-otaAssignChn
		leastOccupiedBucketID := 0
		for bucketID, keyLength := range bucketInfo {
			if keyLength < bucketInfo[leastOccupiedBucketID] {
				leastOccupiedBucketID = bucketID
			}
		}
		request.Key.BucketID = leastOccupiedBucketID
		err := DBSaveOTAKey([]SubmittedOTAKeyData{*request.Key})
		if err != nil {
			go func() {
				request.Respond <- err
			}()
		} else {
			bucketInfo[leastOccupiedBucketID] += 1
			request.Respond <- nil
			if bucketInfo[leastOccupiedBucketID] >= 1000 {
				log.Printf("Bucket %v has more than 1000 keys", leastOccupiedBucketID)
			}
		}
	}
}
