package otaindexer

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/incognitochain/coin-service/database"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

func StartOTAIndexingFull() {
	log.Println("initiating ota-indexing-service...")
	cleanAssignedOTA()
	loadSubmittedOTAKey()
	for _, v := range Submitted_OTAKey.Keys {
		assignedOTAKeys.Lock()
		pubkey, _, err := base58.Base58Check{}.Decode(v.Pubkey)
		if err != nil {
			log.Fatalln(err)
		}
		keyBytes, _, err := base58.Base58Check{}.Decode(v.OTAKey)
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
		data, err := database.DBGetCoinV2PubkeyInfo(v.Pubkey)
		if err != nil {
			log.Fatalln(err)
		}
		data.OTAKey = v.OTAKey
		k := OTAkeyInfo{
			KeyInfo: data,
			ShardID: int(shardID),
			OTAKey:  v.OTAKey,
			Pubkey:  v.Pubkey,
			keyset:  ks,
		}
		assignedOTAKeys.Keys[int(shardID)] = append(assignedOTAKeys.Keys[int(shardID)], &k)
		assignedOTAKeys.TotalKeys += 1
		assignedOTAKeys.Unlock()
	}
	interval := time.NewTicker(6 * time.Second)
	var coinList []shared.CoinData
	go func() {
		for {
			request := <-OTAAssignChn
			key := request.Key
			Submitted_OTAKey.RLock()
			if _, ok := Submitted_OTAKey.Keys[key.Pubkey]; ok {
				go func() {
					request.Respond <- fmt.Errorf("key %v already exist", key.Pubkey)
				}()
				Submitted_OTAKey.RUnlock()
				continue
			}
			Submitted_OTAKey.RUnlock()

			key.IndexerID = shared.ServiceCfg.IndexerID
			err := database.DBSaveSubmittedOTAKeys([]shared.SubmittedOTAKeyData{*key})
			if err != nil {
				go func() {
					request.Respond <- err
				}()
				continue
			}
			err = addKeys([]shared.SubmittedOTAKeyData{*key}, false)
			if err != nil {
				go func() {
					request.Respond <- err
				}()
				continue
			}
			go func() {
				request.Respond <- nil
			}()
			//
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
	}()

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
		lastPRVIndex, lastTokenIndex := GetOTAKeyListMinScannedCoinIndex(assignedOTAKeys.Keys)
		var filteredCoins map[string][]shared.CoinData
		for {
			coinList = GetUnknownCoinsFromDB(lastPRVIndex, lastTokenIndex)
			if len(coinList) == 0 {
				break
			}
			filteredCoins, _, lastPRVIndex, lastTokenIndex, err = filterCoinsByOTAKey(coinList)
			if err != nil {
				panic(err)
			}
			updateCoinState(filteredCoins, lastPRVIndex, lastTokenIndex)
		}
		log.Println("finish scanning coins in", time.Since(startTime))
		assignedOTAKeys.Unlock()
	}
}
