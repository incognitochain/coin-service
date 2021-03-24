package main

import (
	"encoding/hex"

	"errors"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"sync"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/transaction/utils"
	"github.com/incognitochain/incognito-chain/wallet"
)

func startAPIService() {
	log.Println("initiating api-service...")
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/submitotakey", submitOTAkeyHandler)
	http.HandleFunc("/getcoins", getCoinsHandler)
	http.HandleFunc("/checkkeyimages", checkKeyImagesHandler)
	http.HandleFunc("/getrandomcommitments", getRandomCommitmentsHandler)
	http.HandleFunc("/getkeyinfo", getKeyInfoHandler)
	http.HandleFunc("/getcoinspending", getCoinsPendingHandler)
	err := http.ListenAndServe("0.0.0.0:"+strconv.Itoa(serviceCfg.APIPort), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getCoinsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	version, _ := strconv.Atoi(r.URL.Query().Get("version"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	tokenid := r.URL.Query().Get("tokenid")
	if tokenid == "" {
		tokenid = common.PRVCoinID.String()
	}
	if version != 1 && version != 2 {
		version = 1
	}
	var result []interface{}
	var pubkey string
	highestIdx := uint64(0)
	if version == 2 {
		otakey := r.URL.Query().Get("otakey")
		if otakey != "" {
			wl, err := wallet.Base58CheckDeserialize(otakey)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			if wl.KeySet.OTAKey.GetOTASecretKey() == nil {
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(buildErrorRespond(errors.New("invalid otakey")))
				if err != nil {
					log.Println(err)
				}
				return
			}
			pubkey = base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
			tokenidv2 := tokenid
			if tokenid != common.PRVCoinID.String() && tokenid != common.ConfidentialAssetID.String() {
				tokenidv2 = common.ConfidentialAssetID.String()
			}
			coinList, err := DBGetCoinsByOTAKeyAndHeight(tokenidv2, hex.EncodeToString(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), offset, limit)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}

			for _, c := range coinList {
				if c.CoinIndex > highestIdx {
					highestIdx = c.CoinIndex
				}
				coinV2 := new(coin.CoinV2)
				err := coinV2.SetBytes(c.Coin)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_, err = w.Write(buildErrorRespond(err))
					if err != nil {
						log.Println(err)
					}
					return
				}
				idx := new(big.Int).SetUint64(c.CoinIndex)
				cn := jsonresult.NewOutCoin(coinV2)
				cn.Index = base58.Base58Check{}.Encode(idx.Bytes(), common.ZeroByte)
				result = append(result, cn)
			}
		}
	}
	if version == 1 {
		viewkey := r.URL.Query().Get("viewkey")
		var viewKeySet *incognitokey.KeySet
		if viewkey == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write(buildErrorRespond(errors.New("viewkey cant be empty")))
			if err != nil {
				log.Println(err)
			}
			return
		}
		wl, err := wallet.Base58CheckDeserialize(viewkey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		// fmt.Println(wl.Base58CheckSerialize(wallet.PaymentAddressType))
		// fmt.Println(wl.Base58CheckSerialize(wallet.ReadonlyKeyType))
		if wl.KeySet.ReadonlyKey.Rk == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(errors.New("invalid viewkey")))
			if err != nil {
				log.Println(err)
			}
			return
		}
		pubkey = base58.EncodeCheck(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		wl.KeySet.PaymentAddress.Pk = wl.KeySet.ReadonlyKey.Pk
		viewKeySet = &wl.KeySet
		coinListV1, err := DBGetCoinV1ByPubkey(tokenid, hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS()), int64(offset), int64(limit))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		var wg sync.WaitGroup
		collectCh := make(chan OutCoinV1, MAX_CONCURRENT_COIN_DECRYPT)
		decryptCount := 0
		for idx, c := range coinListV1 {
			if c.CoinIndex > highestIdx {
				highestIdx = c.CoinIndex
			}
			coinV1 := new(coin.CoinV1)
			err := coinV1.SetBytes(c.Coin)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			if viewKeySet != nil {
				wg.Add(1)
				go func(cn *coin.CoinV1, cData CoinData) {
					plainCoin, err := cn.Decrypt(viewKeySet)
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						_, err = w.Write(buildErrorRespond(err))
						if err != nil {
							log.Println(err)
						}
						return
					}
					cV1 := NewOutCoinV1(plainCoin)
					idx := new(big.Int).SetUint64(cData.CoinIndex)
					cV1.Index = base58.EncodeCheck(idx.Bytes())
					collectCh <- cV1
					wg.Done()
				}(coinV1, c)
				if decryptCount%MAX_CONCURRENT_COIN_DECRYPT == 0 || idx+1 == len(coinListV1) {
					wg.Wait()
					close(collectCh)
					for coin := range collectCh {
						result = append(result, coin)
					}
					collectCh = make(chan OutCoinV1, MAX_CONCURRENT_COIN_DECRYPT)
				}
			} else {
				cn := NewOutCoinV1(coinV1)
				result = append(result, cn)
			}
		}
	}

	rs := make(map[string]interface{})
	rs["HighestIndex"] = highestIdx
	rs["Outputs"] = map[string]interface{}{pubkey: result}
	respond := API_respond{
		Result: rs,
		Error:  nil,
	}
	respondBytes, err := json.Marshal(respond)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(200)
	_, err = w.Write(respondBytes)
	if err != nil {
		log.Println(err)
	}
	return
}

func checkKeyImagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	var req API_check_keyimages_request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}

	result, err := DBCheckKeyimagesUsed(req.Keyimages, req.ShardID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(200)
	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	respondBytes, err := json.Marshal(respond)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	_, err = w.Write(respondBytes)
	if err != nil {
		log.Println(err)
	}
	return
}

func submitOTAkeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	var req API_submit_otakey_request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	err = addOTAKey(req.OTAKey, 0, 0, req.ShardID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("ok"))
	if err != nil {
		log.Println(err)
		return
	}
	return
}

func getRandomCommitmentsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var req API_get_random_commitment_request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}

	commitmentIndices := []uint64{}
	myIndices := []uint64{}
	var publicKeys, commitments, assetTags []string

	if req.Version == 1 && len(req.Indexes) > 0 {
		result, err := DBGetCoinV1ByIndexes(req.Indexes, req.ShardID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		listUsableCommitments := make(map[common.Hash][]byte)
		listUsableCommitmentsIndices := make([]common.Hash, len(req.Indexes))
		mapIndexCommitmentsInUsableTx := make(map[string]*big.Int)
		usableInputCoins := []*coin.CoinV1{}
		for i, c := range result {
			coinV1 := new(coin.CoinV1)
			coinV1.SetBytes(c.Coin)
			usableInputCoins = append(usableInputCoins, coinV1)
			usableCommitment := coinV1.GetCommitment().ToBytesS()
			commitmentInHash := common.HashH(usableCommitment)
			listUsableCommitments[commitmentInHash] = usableCommitment
			listUsableCommitmentsIndices[i] = commitmentInHash
			commitmentInBase58Check := base58.Base58Check{}.Encode(usableCommitment, common.ZeroByte)
			mapIndexCommitmentsInUsableTx[commitmentInBase58Check] = new(big.Int).SetUint64(c.CoinIndex)
		}

		// loop to random commitmentIndexs
		cpRandNum := (len(listUsableCommitments) * privacy.CommitmentRingSize) - len(listUsableCommitments)
		//fmt.Printf("cpRandNum: %d\n", cpRandNum)
		lenCommitment := new(big.Int).SetInt64(DBGetCoinV1OfShardCount(req.ShardID, req.TokenID))
		if lenCommitment == nil {
			utils.Logger.Log.Error(errors.New("Commitments is empty"))
			return
		}
		if lenCommitment.Uint64() == 1 && len(req.Indexes) == 1 {
			commitmentIndices = []uint64{0, 0, 0, 0, 0, 0, 0}
			temp := base58.EncodeCheck(usableInputCoins[0].GetCommitment().ToBytesS())
			commitments = []string{temp, temp, temp, temp, temp, temp, temp}
		} else {
			randIdxs := []uint64{}
			for i := 0; i < cpRandNum; i++ {
			random:
				index, _ := common.RandBigIntMaxRange(lenCommitment)
				for _, v := range mapIndexCommitmentsInUsableTx {
					if index.Uint64() == v.Uint64() {
						goto random
					}
				}
				randIdxs = append(randIdxs, index.Uint64())
			}

			coinList, err := DBGetCoinV1ByIndexes(randIdxs, req.ShardID)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			if len(randIdxs) != len(coinList) {
				w.WriteHeader(http.StatusInternalServerError)
				_, err = w.Write(buildErrorRespond(errors.New("len(randIdxs) != len(coinList)")))
				if err != nil {
					log.Println(err)
				}
				return
			}
			for _, c := range coinList {
				coinV1 := new(coin.CoinV1)
				coinV1.SetBytes(c.Coin)
				commitmentIndices = append(commitmentIndices, c.CoinIndex)
				commitments = append(commitments, coinV1.GetCommitment().String())
			}
		}
		// loop to insert usable commitments into commitmentIndexs for every group
		j := 0
		for _, commitmentInHash := range listUsableCommitmentsIndices {
			commitmentValue := base58.Base58Check{}.Encode(listUsableCommitments[commitmentInHash], common.ZeroByte)
			index := mapIndexCommitmentsInUsableTx[commitmentValue]
			randInt := rand.Intn(privacy.CommitmentRingSize)
			i := (j * privacy.CommitmentRingSize) + randInt
			commitmentIndices = append(commitmentIndices[:i], append([]uint64{index.Uint64()}, commitmentIndices[i:]...)...)
			commitments = append(commitments[:i], append([]string{commitmentValue}, commitments[i:]...)...)
			myIndices = append(myIndices, uint64(i)) // create myCommitmentIndexs
			j++
		}

	}
	if req.Version == 2 && req.Limit > 0 {
		if req.TokenID != common.PRVCoinID.String() {
			req.TokenID = common.ConfidentialAssetID.String()
		}
		lenOTA := new(big.Int).SetInt64(DBGetCoinV2OfShardCount(req.ShardID, req.TokenID) - 1)
		var hasAssetTags bool = true
		for i := 0; i < req.Limit; i++ {
			idx, _ := common.RandBigIntMaxRange(lenOTA)
			log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
			coinData, err := DBGetCoinsByIndex(int(idx.Int64()), req.ShardID, req.TokenID)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			coinV2 := new(coin.CoinV2)
			err = coinV2.SetBytes(coinData.Coin)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			publicKey := coinV2.GetPublicKey()
			commitment := coinV2.GetCommitment()

			commitmentIndices = append(commitmentIndices, idx.Uint64())
			publicKeys = append(publicKeys, base58.EncodeCheck(publicKey.ToBytesS()))
			commitments = append(commitments, base58.EncodeCheck(commitment.ToBytesS()))

			if hasAssetTags {
				assetTag := coinV2.GetAssetTag()
				if assetTag != nil {
					assetTags = append(assetTags, base58.EncodeCheck(assetTag.ToBytesS()))
				} else {
					hasAssetTags = false
				}
			}
		}

	}

	rs := make(map[string]interface{})
	rs["CommitmentIndices"] = commitmentIndices
	rs["MyIndices"] = myIndices
	rs["PublicKeys"] = publicKeys
	rs["Commitments"] = commitments
	rs["AssetTags"] = assetTags
	respond := API_respond{
		Result: rs,
		Error:  nil,
	}
	respondBytes, err := json.Marshal(respond)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	_, err = w.Write(respondBytes)
	if err != nil {
		log.Println(err)
	}
	return
}

func getKeyInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	key := r.URL.Query().Get("key")
	if key != "" {
		wl, err := wallet.Base58CheckDeserialize(key)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		pubkey := hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		result, err := DBGetCoinPubkeyInfo(pubkey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		respond := API_respond{
			Result: result,
			Error:  nil,
		}
		respondBytes, err := json.Marshal(respond)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		_, err = w.Write(respondBytes)
		if err != nil {
			log.Println(err)
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write(buildErrorRespond(errors.New("key cant be empty")))
		if err != nil {
			log.Println(err)
		}
	}
}

func getCoinsPendingHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}

	result, err := DBGetPendingCoins()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	respond := API_respond{
		Result: result,
		Error:  nil,
	}
	respondBytes, err := json.Marshal(respond)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	_, err = w.Write(respondBytes)
	if err != nil {
		log.Println(err)
	}
	return
}

func buildErrorRespond(err error) []byte {
	errStr := err.Error()
	respond := API_respond{
		Result: nil,
		Error:  &errStr,
	}
	respondBytes, _ := json.Marshal(respond)
	return respondBytes
}
