package main

import (
	"encoding/json"
	"log"
	"net/http"
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
	key := r.URL.Query().Get("otakey")

	var result []interface{}
	coinList, err := DBGetCoinsByOTAKey(key)
	if err != nil {
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}
	for _, c := range coinList {
		result = append(result, c)
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(200)
	_, err = w.Write(resultBytes)
	if err != nil {
		panic(err)
	}
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

	result, err := DBCheckKeyimagesUsed(req.Keyimages, req.ShardID)
	if err != nil {
		http.Error(w, "Unexpected error", http.StatusBadRequest)
		return
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}
	w.Write(resultBytes)
	w.WriteHeader(200)
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
	err = addOTAKey(req.OTAKey, req.BeaconHeight, req.ShardID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(200)
	_, err = w.Write([]byte("ok"))
	if err != nil {
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}
	return
}
