package main

import (
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"net/http"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
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
	key := r.URL.Query().Get("otakey")

	fromHeight, _ := strconv.Atoi(r.URL.Query().Get("fromheight"))
	toHeight, _ := strconv.Atoi(r.URL.Query().Get("toheight"))

	var result []jsonresult.OutCoin
	coinList, err := DBGetCoinsByOTAKeyAndHeight(key, fromHeight, toHeight)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write(buildErrorRespond(err))
		if err != nil {
			log.Println(err)
		}
		return
	}

	for _, c := range coinList {
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
	rs := make(map[string]map[string]interface{})
	rs["Outputs"] = make(map[string]interface{})
	rs["Outputs"][key] = result
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
	err = addOTAKey(req.OTAKey, req.BeaconHeight, req.ShardID)
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
	if token == "" {
		token = common.PRVCoinID.String()
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
