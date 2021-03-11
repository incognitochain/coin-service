package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
)

//manage submitted otakey
type OTAkeyInfo struct {
	RawKey       string
	OTAKey       string
	keyset       *incognitokey.KeySet
	BeaconHeight uint64
}

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys []*OTAkeyInfo
}{}

func loadSubmittedOTAKey() error {
	file, _ := os.Open("./keys.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.Token()
	Submitted_OTAKey.Lock()
	keyInfo := OTAkeyInfo{}
	for decoder.More() {
		decoder.Decode(&keyInfo)
		keyBytes, err := hex.DecodeString(keyInfo.RawKey)
		if err != nil {
			return err
		}
		if len(keyBytes) != 64 {
			return errors.New("keyBytes length isn't 64")
		}
		otaKey := OTAKeyFromRaw(keyBytes)
		ks := &incognitokey.KeySet{}
		ks.OTAKey = otaKey
		keyInfo.keyset = ks
		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, &keyInfo)
	}
	Submitted_OTAKey.Unlock()
	return nil
}
func saveSubmittedOTAKey() error {
	Submitted_OTAKey.Lock()
	defer Submitted_OTAKey.Unlock()
	file, err := os.OpenFile("keys.json", os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	err = encoder.Encode(Submitted_OTAKey.Keys)
	return err
}

func addOTAKey(key string, beaconHeight uint64) error {
	Submitted_OTAKey.RLock()
	for _, keyInfo := range Submitted_OTAKey.Keys {
		if keyInfo.RawKey == key {
			return errors.New("key already added")
		}
	}
	Submitted_OTAKey.RUnlock()

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return err
	}
	if len(keyBytes) != 64 {
		return errors.New("keyBytes length isn't 64")
	}
	otaKey := OTAKeyFromRaw(keyBytes)
	ks := &incognitokey.KeySet{}
	ks.OTAKey = otaKey
	keyInfo := &OTAkeyInfo{
		RawKey:       key,
		OTAKey:       hex.EncodeToString(ks.OTAKey.GetOTASecretKey().ToBytesS()),
		BeaconHeight: beaconHeight,
		keyset:       ks,
	}
	Submitted_OTAKey.Lock()
	Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, keyInfo)
	Submitted_OTAKey.Unlock()
	err = saveSubmittedOTAKey()
	if err != nil {
		return err
	}
	log.Println("add key success")
	return nil
}

func getUnknowCoinsFromDB() []*CoinData {
	var result []*CoinData
	return result
}

func initOTAIndexingService() {
	log.Println("initiating ota-indexing-service...")

	// 112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt
	// 111111bgk2j6vZQvzq8tkonDLLXEvLkMwBMn5BoLXLpf631boJnPDGEQMGvA1pRfT71Crr7MM2ShvpkxCBWBL2icG22cXSpcKybKCQmaxa
	// acc0, _ := account.NewAccountFromPrivatekey("112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt")
	// key := []byte{}
	// key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	// key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	// addOTAKey(hex.EncodeToString(key), 0)
	// addOTAKey(hex.EncodeToString(key), 0)
	// addOTAKey(hex.EncodeToString(key), 0)
	// addOTAKey(hex.EncodeToString(key), 0)
	// addOTAKey(hex.EncodeToString(key), 0)

	err := loadSubmittedOTAKey()
	if err != nil {
		panic(err)
	}
	interval := time.NewTicker(10 * time.Second)
	for {
		<-interval.C
	}
}

func filterCoinsByOTAKey(coinList []CoinData) (map[string][]CoinData, []CoinData, error) {
	if len(Submitted_OTAKey.Keys) > 0 {
		otaCoins := make(map[string][]CoinData)
		var otherCoins []CoinData
		startTime := time.Now()
		Submitted_OTAKey.RLock()
		for _, c := range coinList {
			newCoin := new(coin.CoinV2)
			err := newCoin.SetBytes(c.Coin)
			if err != nil {
				panic(err)
			}
			pass := false
			for _, keyInfo := range Submitted_OTAKey.Keys {
				if c.BeaconHeight > keyInfo.BeaconHeight {
					pass, _ = newCoin.DoesCoinBelongToKeySet(keyInfo.keyset)
					if pass {
						otaCoins[keyInfo.OTAKey] = append(otaCoins[keyInfo.OTAKey], c)
						break
					}
				}
			}
			if !pass {
				otherCoins = append(otherCoins, c)
			}
		}
		Submitted_OTAKey.RUnlock()

		log.Printf("filtered %v coins with %v keys in %v", len(coinList), len(Submitted_OTAKey.Keys), time.Since(startTime))
		return otaCoins, otherCoins, nil
	}
	return nil, nil, nil
}
