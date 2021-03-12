package main

import (
	"fmt"

	devframework "github.com/0xkumi/incognito-dev-framework"
)

func main() {
	readConfig()
	node := devframework.NewAppNode("fullnode", devframework.TestNetParam, true, false)
	localnode = node
	err := connectDB()
	if err != nil {
		panic(err)
	}
	fmt.Println("Database Connected!")
	initCoinService()
	go initOTAIndexingService()
	go startAPIService(DefaultAPIAddress)

	// list := []CoinData{*NewCoinData(1, 1, []byte("test"), "abc", "xyz", "bcd", "jkl")}
	// for i := 0; i < 100; i++ {
	// 	list = append(list, *NewCoinData(1, uint64(time.Now().Unix()), []byte("test"), "abc", "xyz", "bcd", "jkl"))
	// }
	// for i := 0; i < 100; i++ {
	// 	list = append(list, *NewCoinData(1, uint64(time.Now().Unix()), []byte("test"), "abc", "xyz", "", "jkl"))
	// }
	// err = DBSaveCoins(list)
	// fmt.Println(err)
	// c, err := DBGetCoins("")
	// fmt.Println(len(c), err)
	// c2, err := DBGetCoins("bcd")
	// fmt.Println(len(c2), err)
	// newList := []CoinData{}
	// for _, coin := range c2 {
	// 	coin.CoinIndex = 0
	// 	newList = append(newList, coin)
	// }
	// err = DBUpdateCoins(newList)
	// fmt.Println(err)
	select {}
}
