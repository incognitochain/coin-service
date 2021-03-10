package main

import (
	"encoding/hex"
	"errors"
	"log"
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
	keys := []OTAkeyInfo{}
	Submitted_OTAKey.Lock()
	for _, keyInfo := range keys {
		keyBytes, err := hex.DecodeString(keyInfo.OTAKey)
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
	return nil
}

func addOTAKey(key string, beaconHeight uint64) error {
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
	return nil
}

func getUnknowCoinsFromDB() []*CoinData {
	var result []*CoinData
	return result
}

func initOTAIndexingService() {
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
