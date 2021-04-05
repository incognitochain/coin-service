package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
)

//manage submitted otakey
type OTAkeyInfo struct {
	ShardID        int
	Pubkey         string
	OTAKey         string
	keyset         *incognitokey.KeySet
	LastPRVIndex   CoinInfo
	LastTokenIndex CoinInfo
}

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys []*OTAkeyInfo
}{}

func loadSubmittedOTAKey() error {
	keys, err := DBGetOTAKeys(serviceCfg.IndexerBucketID)
	if err != nil {
		return err
	}
	Submitted_OTAKey.Lock()
	for _, key := range keys {
		pubkey, _, _ := base58.Base58Check{}.Decode(key.Pubkey)
		keyBytes, _, _ := base58.Base58Check{}.Decode(key.OTAKey)
		keyBytes = append(keyBytes, pubkey...)
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		otaKey := OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
		data, err := DBGetCoinV1PubkeyInfo(key.Pubkey)
		if err != nil {
			return err
		}
		k := OTAkeyInfo{
			LastPRVIndex:   data.CoinIndex[common.PRVCoinID.String()],
			LastTokenIndex: data.CoinIndex[common.ConfidentialAssetID.String()],
			ShardID:        int(shardID),
			OTAKey:         key.OTAKey,
			Pubkey:         key.Pubkey,
			keyset:         ks,
		}
		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &k)
	}
	Submitted_OTAKey.Unlock()
	pipeline := []bson.M{
		{"bucketid": bson.M{operator.Eq: serviceCfg.IndexerBucketID}},
	}
	changeStream, err := mgm.Coll(&SubmittedOTAKeyData{}).Watch(context.Background(), pipeline)
	if err != nil {
		return err
	}
	for changeStream.Next(context.Background()) {
		Submitted_OTAKey.Lock()
		key := SubmittedOTAKeyData{}
		if err := changeStream.Decode(key); err != nil {
			log.Fatal(err)
		}
		pubkey, _, _ := base58.Base58Check{}.Decode(key.Pubkey)
		keyBytes, _, _ := base58.Base58Check{}.Decode(key.OTAKey)
		keyBytes = append(keyBytes, pubkey...)
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		otaKey := OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		shardID := common.GetShardIDFromLastByte(pubkey[len(pubkey)-1])
		data, err := DBGetCoinV1PubkeyInfo(key.Pubkey)
		if err != nil {
			return err
		}
		k := OTAkeyInfo{
			LastPRVIndex:   data.CoinIndex[common.PRVCoinID.String()],
			LastTokenIndex: data.CoinIndex[common.ConfidentialAssetID.String()],
			ShardID:        int(shardID),
			OTAKey:         key.OTAKey,
			Pubkey:         key.Pubkey,
			keyset:         ks,
		}
		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &k)
		Submitted_OTAKey.Unlock()
	}

	if err := changeStream.Close(context.Background()); err != nil {
		return err
	}
	return nil
}

func updateSubmittedOTAKey() error {
	Submitted_OTAKey.Lock()

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

	updateState := func() {
		Submitted_OTAKey.Lock()
		for _, keyInfo := range Submitted_OTAKey.Keys {
			if keyInfo.LastPRVIndex.End < lastPRVIndex {
				keyInfo.LastPRVIndex.End = lastPRVIndex
			}
			if keyInfo.LastTokenIndex.End < lastTokenIndex {
				keyInfo.LastTokenIndex.End = lastTokenIndex
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

		minPRVIndex, minTokenIndex := GetOTAKeyListMinCoinIndex()

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
					updateState()
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
		updateState()
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
				for _, keyInfo := range Submitted_OTAKey.Keys {
					if newCoin.GetAssetTag() == nil {
						if cn.CoinIndex <= keyInfo.LastPRVIndex.End {
							continue
						}
					} else {
						if cn.CoinIndex <= keyInfo.LastTokenIndex.End {
							continue
						}
					}
					pass, _ = newCoin.DoesCoinBelongToKeySet(keyInfo.keyset)
					if pass {
						tempOTACoinsCh <- map[string]CoinData{keyInfo.OTAKey: cn}
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
		return otaCoins, otherCoins, lastPRVIndex, lastTokenIndex, nil
	}
	return nil, nil, 0, 0, nil
}

func GetOTAKeyListMinCoinIndex() (uint64, uint64) {
	Submitted_OTAKey.RLock()
	minPRVIdx := uint64(Submitted_OTAKey.Keys[0].LastPRVIndex.End)
	minTokenIdx := uint64(Submitted_OTAKey.Keys[0].LastTokenIndex.End)
	for _, keyInfo := range Submitted_OTAKey.Keys {
		if minPRVIdx > keyInfo.LastPRVIndex.End {
			minPRVIdx = keyInfo.LastPRVIndex.End
		}
		if minTokenIdx > keyInfo.LastTokenIndex.End {
			minTokenIdx = keyInfo.LastTokenIndex.End
		}
	}
	Submitted_OTAKey.RUnlock()
	return minPRVIdx, minTokenIdx
}

func GetCoinsFromDB(fromPRVIndex, fromTokenIndex uint64) []CoinData {
	log.Println("requesting 500 prv from index", fromPRVIndex)
	coinList, err := DBGetUnknownCoinsFromCoinIndexWithLimit(fromPRVIndex, true, 500)
	if err != nil {
		panic(err)
	}
	log.Println("requesting 500 token from index", fromTokenIndex)
	coinListTk, err := DBGetUnknownCoinsFromCoinIndexWithLimit(fromTokenIndex, false, 500)
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
	bucketInfo, err := DBGetBucketStats(serviceCfg.NumberOfBucket)
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
