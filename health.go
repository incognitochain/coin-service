package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, err := w.Write(buildErrorRespond(errors.New("Method not allowed")))
		if err != nil {
			log.Println(err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	resp, _ := json.Marshal(map[string]string{"status": "ok"})
	_, err := w.Write(resp)
	if err != nil {
		log.Println(err)
	}
	return
}
