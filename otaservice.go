package main

import (
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
)

//manage submitted otakey

var Submitted_OTAKey = struct {
	sync.RWMutex
	Keys    []string
	OTAs    []string
	Keysets []*incognitokey.KeySet
}{}

func loadSubmittedOTAKey() error {
	keys := []string{}
	Submitted_OTAKey.Lock()
	for _, key := range keys {
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

		Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, key)
		Submitted_OTAKey.Keysets = append(Submitted_OTAKey.Keysets, ks)
		Submitted_OTAKey.OTAs = append(Submitted_OTAKey.OTAs, hex.EncodeToString(ks.OTAKey.GetOTASecretKey().ToBytesS()))
	}
	Submitted_OTAKey.Unlock()
	return nil
}
func saveSubmittedOTAKey() error {
	return nil
}

func addOTAKey(key string) error {
	Submitted_OTAKey.Lock()
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

	Submitted_OTAKey.Keys = append(Submitted_OTAKey.Keys, key)
	Submitted_OTAKey.Keysets = append(Submitted_OTAKey.Keysets, ks)
	Submitted_OTAKey.OTAs = append(Submitted_OTAKey.OTAs, hex.EncodeToString(ks.OTAKey.GetOTASecretKey().ToBytesS()))
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

		Submitted_OTAKey.RLock()
		for _, c := range coinList {
			newCoin := new(coin.CoinV2)
			err := newCoin.SetBytes(c.Coin)
			if err != nil {
				panic(err)
			}
			for idx, keyset := range Submitted_OTAKey.Keysets {
				pass, _ := newCoin.DoesCoinBelongToKeySet(keyset)
				if pass {
					otaCoins[Submitted_OTAKey.OTAs[idx]] = append(otaCoins[Submitted_OTAKey.OTAs[idx]], c)
				}
			}

		}
		Submitted_OTAKey.RUnlock()
		return otaCoins, otherCoins, nil
	}
	return nil, nil, nil
}
