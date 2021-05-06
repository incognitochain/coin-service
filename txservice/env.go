package main

import (
	"log"
	"os"
)

var (
	GGC_PROJECT = mustGetenv("GOOGLE_CLOUD_PROJECT")
	TX_TOPIC    = mustGetenv("TX_TOPIC")
	MONGODB     = mustGetenv("MONGO")
	DBNAME      = mustGetenv("DBNAME")
	FULLNODE    = mustGetenv("FULLNODE")
	MODE        = mustGetenv("MODE")
	PORT        = mustGetenv("PORT")
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("%s environment variable not set.", k)
	}
	return v
}
