package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func startAPIService(address string) {
	log.Println("initiating api-service...")
	http.HandleFunc("/submitotakey", submitOTAkeyHandler)
	http.HandleFunc("/getcoins", getCoinsHandler)
	http.HandleFunc("/checkkeyimages", checkKeyImagesHandler)
	err := http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getCoinsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// accName := r.URL.Query().Get("otakey")
	// accountListLck.RLock()
	// defer accountListLck.RUnlock()
	// if _, ok := accountList[accName]; !ok {
	// 	http.Error(w, "account name isn't exist", http.StatusBadRequest)
	// 	return
	// }
	// // if !accountList[accName].isReady {
	// // 	http.Error(w, "account not ready", http.StatusBadRequest)
	// // 	return
	// // }
	// accState := accountList[accName]
	// accState.lock.RLock()
	// encryptCoins := make(map[string]map[string]string)
	// fmt.Println("EncryptedCoins", accState.coinState.EncryptedCoins)
	// for tokenID, coinsPubkey := range accState.coinState.EncryptedCoins {
	// 	if len(coinsPubkey) > 0 {
	// 		fmt.Println("len(coinsPubkey)", len(coinsPubkey))
	// 		coins, err := getCoinsByCoinPubkey(accState.Account.PAstr, tokenID, coinsPubkey)
	// 		if err != nil {
	// 			accState.lock.RUnlock()
	// 			http.Error(w, "Unexpected error", http.StatusInternalServerError)
	// 			return
	// 		}
	// 		otakey := &key.OTAKey{}
	// 		otakey.SetOTASecretKey(accState.Account.OTAKey)
	// 		encryptCoins[tokenID], err = ExtractCoinEncryptKeyImgData(coins, otakey)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 	}
	// }
	// accState.lock.RUnlock()
	// if len(encryptCoins) == 0 {
	// 	http.Error(w, "no coin needed to decrypt", http.StatusBadRequest)
	// 	return
	// }
	// coinsBytes, err := json.Marshal(encryptCoins)
	// if err != nil {
	// 	http.Error(w, "Unexpected error", http.StatusInternalServerError)
	// 	return
	// }
	// w.WriteHeader(200)
	// _, err = w.Write(coinsBytes)
	// if err != nil {
	// 	panic(err)
	// }
	return
}

func checkKeyImagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req API_check_keyimages_request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// accountListLck.RLock()
	// accountState, ok := accountList[req.Account]
	// accountListLck.RUnlock()
	// if !ok {
	// 	http.Error(w, "account name isn't exist", http.StatusBadRequest)
	// 	return
	// }
	// coinList := make(map[string][]string)
	// keyimages := make(map[string]string)
	// for token, coinsKm := range req.Keyimages {
	// 	for coinPK, km := range coinsKm {
	// 		coinList[token] = append(coinList[token], coinPK)
	// 		keyimages[coinPK] = km
	// 	}
	// }
	// err = accountState.UpdateDecryptedCoin(coinList, keyimages)
	// if err != nil {
	// 	panic(err)
	// }
	// w.WriteHeader(200)
	return
}

func submitOTAkeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req API_submit_otakey_request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	return
}
