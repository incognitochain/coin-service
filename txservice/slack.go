package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func sendSlackNoti(text string) {
	content := struct {
		Text string `json:"text"`
	}{
		Text: text,
	}
	contentBytes, err := json.Marshal(content)
	if err != nil {
		log.Println(err)
		return
	}
	httpClient := http.DefaultClient
	resp, err := httpClient.Post(SLACKHOOK, "application/json", bytes.NewReader(contentBytes))
	if resp.Status != "200" || err != nil {
		log.Println(err)
		body, _ := ioutil.ReadAll(resp.Body)
		log.Println(string(body))
	}
	defer resp.Body.Close()
}
