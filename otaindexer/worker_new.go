package otaindexer

import (
	"log"
	"sync"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/privacy/coin"
)

func filterCoinsByOTAKeyV2(coinList []shared.CoinData, keys []*OTAkeyInfo, tokenIDMap map[string]string, nftIDMap map[string]string) ([]shared.CoinData, map[string]map[string][]string) {
	startTime := time.Now()
	indexedCoins := []shared.CoinData{}

	totalTxs := make(map[string]map[string]map[string]struct{})
	txsToUpdate := make(map[string]map[string][]string)

	var wg sync.WaitGroup
	tempOTACoinsCh := make(chan map[string]shared.CoinData, shared.ServiceCfg.MaxConcurrentOTACheck)
	var keyLock sync.Mutex
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
			for _, keyData := range keys {
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
					keyLock.Lock()
					if !isNFT {
						if _, ok := keyData.KeyInfo.CoinIndex[cn.RealTokenID]; !ok {
							keyData.KeyInfo.CoinIndex[cn.RealTokenID] = shared.CoinInfo{
								Start: cn.CoinIndex,
								End:   cn.CoinIndex,
								Total: 1,
							}
						} else {
							d := keyData.KeyInfo.CoinIndex[cn.RealTokenID]
							if d.Total == 0 {
								d.Start = cn.CoinIndex
								d.End = cn.CoinIndex
							}
							if d.Start > cn.CoinIndex {
								d.Start = cn.CoinIndex
							}
							if d.End < cn.CoinIndex {
								d.End = cn.CoinIndex
							}
							d.Total += 1
							keyData.KeyInfo.CoinIndex[cn.RealTokenID] = d
						}
					} else {
						if len(keyData.KeyInfo.NFTIndex) == 0 {
							keyData.KeyInfo.NFTIndex = make(map[string]shared.CoinInfo)
						}
						if _, ok := keyData.KeyInfo.NFTIndex[cn.RealTokenID]; !ok {
							keyData.KeyInfo.NFTIndex[cn.RealTokenID] = shared.CoinInfo{
								Start: cn.CoinIndex,
								End:   cn.CoinIndex,
								Total: 1,
							}
						} else {
							d := keyData.KeyInfo.NFTIndex[cn.RealTokenID]
							if d.Total == 0 {
								d.Start = cn.CoinIndex
								d.End = cn.CoinIndex
							}
							if d.Start > cn.CoinIndex {
								d.Start = cn.CoinIndex
							}
							if d.End < cn.CoinIndex {
								d.End = cn.CoinIndex
							}
							d.Total += 1
							keyData.KeyInfo.NFTIndex[cn.RealTokenID] = d
						}
					}
					keyLock.Unlock()
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
						continue
					}
					coin.OTASecret = key
					indexedCoins = append(indexedCoins, coin)
					if _, ok := txsToUpdate[key]; ok {
						txsToUpdate[key][coin.RealTokenID] = append(txsToUpdate[key][coin.RealTokenID], coin.TxHash)
					} else {
						txsToUpdate[key] = make(map[string][]string)
						txsToUpdate[key][coin.RealTokenID] = append(txsToUpdate[key][coin.RealTokenID], coin.TxHash)
					}

					if len(totalTxs[key]) == 0 {
						totalTxs[key] = make(map[string]map[string]struct{})
					}
					if len(totalTxs[key][coin.RealTokenID]) == 0 {
						totalTxs[key][coin.RealTokenID] = make(map[string]struct{})
					}
					totalTxs[key][coin.RealTokenID][coin.TxHash] = struct{}{}
				}
			}
			tempOTACoinsCh = make(chan map[string]shared.CoinData, shared.ServiceCfg.MaxConcurrentOTACheck)
		}
	}
	close(tempOTACoinsCh)
	lastCoinIdx := coinList[len(coinList)-1].CoinIndex

	assignedOTAKeys.Lock()
	for _, key := range keys {
		if len(tokenIDMap) != 0 {
			a := key.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()]
			a.LastScanned = lastCoinIdx
			key.KeyInfo.CoinIndex[common.ConfidentialAssetID.String()] = a
		} else {
			a := key.KeyInfo.CoinIndex[common.PRVCoinID.String()]
			a.LastScanned = lastCoinIdx
			key.KeyInfo.CoinIndex[common.PRVCoinID.String()] = a
		}
	}
	assignedOTAKeys.Unlock()

	log.Printf("finish filter %v coins with %v keys in %v \n", len(coinList), len(keys), time.Since(startTime))
	return indexedCoins, txsToUpdate
}
