package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	wl, _ := wallet.Base58CheckDeserialize("112t8rniZP5hk9X3RjCFx9CXyoxmJFcqM6sNM7Yknng6D4jS3vwTxcQ6hPZ3h3mZHx2JDNxfGxmwjiHN3A34gktcMhgXUwh8EXpo7NCxiuxJ")
	otaKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetOTASecretKey().ToBytesS())
	pubKey := base58.EncodeCheck(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())
	fmt.Println(otaKey, pubKey)
	common.MaxShardNumber = 2
	shardID := common.GetShardIDFromLastByte(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS()[len(wl.KeySet.OTAKey.GetPublicSpend().ToBytesS())-1])
	connectDB()
	list, err := DBGetUnknownCoinsV2(int(shardID), common.PRVCoinID.String(), 0, 0)
	if err != nil {
		panic(8)
	}
	fmt.Println(shardID)
	for _, cn := range list {
		newCoin := new(coin.CoinV2)
		err := newCoin.SetBytes(cn.Coin)
		if err != nil {
			panic(err)
		}
		pass, _ := newCoin.DoesCoinBelongToKeySet(&wl.KeySet)
		if pass {
			panic(9)
		}
	}

}

func connectDB() error {
	err := mgm.SetDefaultConfig(nil, "coin", options.Client().ApplyURI("mongodb://root:example@51.161.119.66:27019"))
	if err != nil {
		return err
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err = cd.Ping(context.Background(), nil)
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func DBGetUnknownCoinsV2(shardID int, tokenID string, fromidx, limit int64) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	if limit == 0 {
		limit = 10000
	}
	filter := bson.M{"otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}, "coinidx": bson.M{operator.Gte: fromidx}}
	err := mgm.Coll(&CoinData{}).SimpleFind(&list, filter, &options.FindOptions{
		// Sort: bson.D{{Key: "coinidx", Value: 1}},
		// Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CoinIndex < list[j].CoinIndex })
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}
