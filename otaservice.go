package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/0xkumi/incognito-dev-framework/account"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
)

//manage submitted otakey
type OTAkeyInfo struct {
	ShardID      int
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
	file, err := os.OpenFile("keys.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	err = encoder.Encode(Submitted_OTAKey.Keys)
	return err
}

func addOTAKey(key string, beaconHeight uint64, shardID int) error {
	Submitted_OTAKey.RLock()
	for _, keyInfo := range Submitted_OTAKey.Keys {
		if keyInfo.RawKey == key {
			Submitted_OTAKey.RUnlock()
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
		ShardID:      shardID,
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

func initOTAIndexingService() {
	log.Println("initiating ota-indexing-service...")

	// 112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt
	// 111111bgk2j6vZQvzq8tkonDLLXEvLkMwBMn5BoLXLpf631boJnPDGEQMGvA1pRfT71Crr7MM2ShvpkxCBWBL2icG22cXSpcKybKCQmaxa

	err := loadSubmittedOTAKey()
	if err != nil {
		panic(err)
	}

	acc0, _ := account.NewAccountFromPrivatekey("112t8rnXoBXrThDTACHx2rbEq7nBgrzcZhVZV4fvNEcGJetQ13spZRMuW5ncvsKA1KvtkauZuK2jV8pxEZLpiuHtKX3FkKv2uC5ZeRC8L6we")
	key := []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))

	acc0, _ = account.NewAccountFromPrivatekey("112t8rnbcZ92v5omVfbXf1gu7j7S1xxr2eppxitbHfjAMHWdLLBjBcQSv1X1cKjarJLffrPGwBhqZzBvEeA9PhtKeM8ALWiWjhUzN5Fi6WVC")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))

	acc0, _ = account.NewAccountFromPrivatekey("112t8rnZUQXxcbayAZvyyZyKDhwVJBLkHuTKMhrS51nQZcXKYXGopUTj22JtZ8KxYQcak54KUQLhimv1GLLPFk1cc8JCHZ2JwxCRXGsg4gXU")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))

	acc0, _ = account.NewAccountFromPrivatekey("112t8rnXDS4cAjFVgCDEw4sWGdaqQSbKLRH1Hu4nUPBFPJdn29YgUei2KXNEtC8mhi1sEZb1V3gnXdAXjmCuxPa49rbHcH9uNaf85cnF3tMw")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))

	acc0, _ = account.NewAccountFromPrivatekey("112t8rnYoioTRNsM8gnUYt54ThWWrRnG4e1nRX147MWGbEazYP7RWrEUB58JLnBjKhh49FMS5o5ttypZucfw5dFYMAsgDUsHPa9BAasY8U1i")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))

	acc0, _ = account.NewAccountFromPrivatekey("112t8roafGgHL1rhAP9632Yef3sx5k8xgp8cwK4MCJsCL1UWcxXvpzg97N4dwvcD735iKf31Q2ZgrAvKfVjeSUEvnzKJyyJD3GqqSZdxN4or")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))
	acc0, _ = account.NewAccountFromPrivatekey("112t8rnZDRztVgPjbYQiXS7mJgaTzn66NvHD7Vus2SrhSAY611AzADsPFzKjKQCKWTgbkgYrCPo9atvSMoCf9KT23Sc7Js9RKhzbNJkxpJU6")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))
	acc0, _ = account.NewAccountFromPrivatekey("112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt")
	key = []byte{}
	key = append(key, acc0.Keyset.OTAKey.GetOTASecretKey().ToBytesS()...)
	key = append(key, acc0.Keyset.OTAKey.GetPublicSpend().ToBytesS()...)
	addOTAKey(hex.EncodeToString(key), 0, acc0.ShardID(2))
	// wl, _ := wallet.Base58CheckDeserialize("112t8rne7fpTVvSgZcSgyFV23FYEv3sbRRJZzPscRcTo8DsdZwstgn6UyHbnKHmyLJrSkvF13fzkZ4e8YD5A2wg8jzUZx6Yscdr4NuUUQDAt")
	// fmt.Println(acc0.PaymentAddress, wl.Base58CheckSerialize(wallet.OTAKeyType))

	interval := time.NewTicker(10 * time.Second)
	for {
		<-interval.C
		log.Println("scanning coins...")
		coinList, err := DBGetUnknownCoinsFromBeaconHeight(0)
		if err != nil {
			panic(err)
		}
		var coinsToUpdate []CoinData
		filteredCoins, _, err := filterCoinsByOTAKey(coinList)
		if err != nil {
			panic(err)
		}
		for key, coins := range filteredCoins {
			for _, coin := range coins {
				coin.OTASecret = key
				coinsToUpdate = append(coinsToUpdate, coin)
			}
		}
		if len(coinsToUpdate) > 0 {
			fmt.Println("\n=========================================")
			log.Println("len(coinsToUpdate)", len(coinsToUpdate))
			fmt.Println("=========================================\n")
			err := DBUpdateCoins(coinsToUpdate)
			if err != nil {
				panic(err)
			}
		}
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
				// if c.BeaconHeight >= keyInfo.BeaconHeight {
				pass, _ = newCoin.DoesCoinBelongToKeySet(keyInfo.keyset)
				if pass {
					otaCoins[keyInfo.OTAKey] = append(otaCoins[keyInfo.OTAKey], c)
					break
				}
				// }
			}
			if !pass {
				otherCoins = append(otherCoins, c)
			}
		}
		Submitted_OTAKey.RUnlock()
		log.Println("len(otaCoins)", len(otaCoins))
		log.Printf("filtered %v coins with %v keys in %v", len(coinList), len(Submitted_OTAKey.Keys), time.Since(startTime))
		return otaCoins, otherCoins, nil
	}
	return nil, nil, nil
}
