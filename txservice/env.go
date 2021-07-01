package main

import (
	"os"
)

var (
	GGC_PROJECT    = mustGetenv("GOOGLE_CLOUD_PROJECT")
	GGC_ACC        = mustGetenv("GOOGLE_CLOUD_ACC")
	TX_TOPIC       = mustGetenv("TX_TOPIC")
	TXSTATUS_TOPIC = mustGetenv("TXSTATUS_TOPIC")
	TX_SUBID       = mustGetenv("TX_SUBID")
	TXSTATUS_SUBID = mustGetenv("TXSTATUS_SUBID")
	MONGODB        = mustGetenv("MONGO")
	DBNAME         = mustGetenv("DBNAME")
	FULLNODE       = mustGetenv("FULLNODE")
	MODE           = mustGetenv("MODE")
	PORT           = mustGetenv("PORT")
	SLACKHOOK      = mustGetenv("SLACKHOOK")
	SHARDNUMBER    = mustGetenv("SHARDNUMBER")
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		// log.Fatalf("%s environment variable not set.", k)
	}
	return v
}
