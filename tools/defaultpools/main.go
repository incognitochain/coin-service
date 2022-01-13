package main

import (
	"fmt"

	"github.com/incognitochain/coin-service/database"
)

func main() {
	readConfig()
	err := database.ConnectDB(cfg.DBName, cfg.Mongo)
	if err != nil {
		panic(err)
	}
	fmt.Println("start")
	switch cfg.Action {
	case add:
		err := database.DBSetDefaultPool(cfg.DataList)
		if err != nil {
			panic(err)
		}
	case remove:
		err := database.DBRemoveDefaultPool(cfg.DataList)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("done")
}
