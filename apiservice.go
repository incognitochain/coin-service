package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/wallet"
)

// var upgrader = websocket.Upgrader{
// 	ReadBufferSize:  1024,
// 	WriteBufferSize: 1024,
// 	CheckOrigin: func(r *http.Request) bool {
// 		return true
// 	},
// }

func startAPIService(address string) {
	log.Println("initiating api-service...")
	http.HandleFunc("/submitotakey", submitOTAkeyHandler)
	http.HandleFunc("/getcoins", getCoinsHandler)
	http.HandleFunc("/checkkeyimages", checkKeyImagesHandler)
	http.HandleFunc("/getrandomcommitments", getRandomCommitmentsHandler)
	err := http.ListenAndServe(address, nil)
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
	fromHeight, _ := strconv.Atoi(r.URL.Query().Get("fromheight"))
	toHeight, _ := strconv.Atoi(r.URL.Query().Get("toheight"))

	tokenid := r.URL.Query().Get("tokenid")
	if tokenid == "" {
		tokenid = common.PRVCoinID.String()
	}
	var result []jsonresult.OutCoin
	var pubkey string
	highestHeight := uint64(0)
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
		pubkey = hex.EncodeToString(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
		tokenidv2 := tokenid
		if tokenid != common.PRVCoinID.String() && tokenid != common.ConfidentialAssetID.String() {
			tokenidv2 = common.ConfidentialAssetID.String()
		}
		coinList, err := DBGetCoinsByOTAKeyAndHeight(tokenidv2, hex.EncodeToString(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS()), fromHeight, toHeight)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}

		for _, c := range coinList {
			if c.BeaconHeight > highestHeight {
				highestHeight = c.BeaconHeight
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

	viewkey := r.URL.Query().Get("viewkey")
	var viewKeySet *incognitokey.KeySet
	if viewkey != "" {
		wl, err := wallet.Base58CheckDeserialize(viewkey)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(err))
			if err != nil {
				log.Println(err)
			}
			return
		}
		if wl.KeySet.ReadonlyKey.Rk == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write(buildErrorRespond(errors.New("invalid viewkey")))
			if err != nil {
				log.Println(err)
			}
			return
		}
		if pubkey == "" {
			pubkey = hex.EncodeToString(wl.KeySet.ReadonlyKey.GetPublicSpend().ToBytesS())
		}
		wl.KeySet.PaymentAddress.Pk = wl.KeySet.ReadonlyKey.Pk
		viewKeySet = &wl.KeySet
	}
	fmt.Println("GetCoinV1", pubkey, viewKeySet)
	coinListV1, err := DBGetCoinV1ByPubkey(tokenid, pubkey, fromHeight, toHeight)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}
	for _, c := range coinListV1 {
		if c.BeaconHeight > highestHeight {
			highestHeight = c.BeaconHeight
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
			plainCoin, err := coinV1.Decrypt(viewKeySet)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, err = w.Write(buildErrorRespond(err))
				if err != nil {
					log.Println(err)
				}
				return
			}
			cn := jsonresult.NewOutCoin(plainCoin)
			result = append(result, cn)
		} else {
			cn := jsonresult.NewOutCoin(coinV1)
			result = append(result, cn)
		}
	}
	rs := make(map[string]interface{})
	rs["HighestHeight"] = highestHeight
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
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}
	shardid, _ := strconv.Atoi(r.URL.Query().Get("shardid"))

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	token := r.URL.Query().Get("token")
	if token == "true" {
		token = common.PRVCoinID.String()
	} else {
		token = common.ConfidentialAssetID.String()
	}

	commitmentIndices := []int{}
	var publicKeys, commitments, assetTags []string

	lenOTA := new(big.Int).SetInt64(DBGetCoinsOfShardCount(shardid, token) - 1)
	var hasAssetTags bool = true
	for i := 0; i < limit; i++ {
		idx, _ := common.RandBigIntMaxRange(lenOTA)
		log.Println("getRandomCommitmentsHandler", lenOTA, idx.Int64())
		coinData, err := DBGetCoinsByIndex(int(idx.Int64()), shardid, token)
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

		commitmentIndices = append(commitmentIndices, int(idx.Int64()))
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

	rs := make(map[string]interface{})
	rs["CommitmentIndices"] = commitmentIndices
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

func buildErrorRespond(err error) []byte {
	errStr := err.Error()
	respond := API_respond{
		Result: nil,
		Error:  &errStr,
	}
	respondBytes, _ := json.Marshal(respond)
	return respondBytes
}
