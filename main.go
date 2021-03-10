package main

import (
	"fmt"
	"time"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	node := devframework.NewAppNode("fullnode", devframework.TestNetParam, true, false)
	localnode = node
	err := connectDB()
	if err != nil {
		panic(err)
	}
	fmt.Println("Database Connected!")
	initCoinService()
	fmt.Println("started service...")

	list := []CoinData{*NewCoinData(1, 1, []byte("test"), "abc", "xyz", "bcd", "jkl")}
	for i := 0; i < 10000; i++ {
		list = append(list, *NewCoinData(1, uint64(time.Now().Unix()), []byte("test"), "abc", "xyz", "bcd", "jkl"))
	}
	for i := 0; i < 10000; i++ {
		list = append(list, *NewCoinData(1, uint64(time.Now().Unix()), []byte("test"), "abc", "xyz", "", "jkl"))
	}
	err = DBSaveCoins(list)
	fmt.Println(err)
	c, err := DBGetCoins("")
	fmt.Println(len(c), err)
	c2, err := DBGetCoins("bcd")
	fmt.Println(len(c2), err)
	select {}
}
