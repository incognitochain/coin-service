package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectDB(dbName string, mongoAddr string) error {
	err := mgm.SetDefaultConfig(nil, dbName, options.Client().ApplyURI(mongoAddr))
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

func DBSaveCoins(list []shared.CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	docsV1 := []interface{}{}
	for _, coin := range list {
		if coin.CoinVersion == 2 {
			docs = append(docs, coin)
		} else {
			docsV1 = append(docsV1, coin)
		}
	}
	if len(docs) > 0 {
		_, err := mgm.Coll(&shared.CoinData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docs), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))
	}
	if len(docsV1) > 0 {
		_, err := mgm.Coll(&shared.CoinDataV1{}).InsertMany(ctx, docsV1)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docsV1), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil
}

func DBSavePendingTx(list []shared.CoinPendingData) error {
	if len(list) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, coin := range list {
			docs = append(docs, coin)
		}
		_, err := mgm.Coll(&shared.CoinPendingData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

func DBDeletePendingTxs(list []string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"txhash": bson.M{operator.In: list}}
	_, err := mgm.Coll(&shared.CoinPendingData{}).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func DBGetPendingTxs() (map[int][]string, error) {
	list := []shared.CoinPendingData{}
	filter := bson.M{}
	err := mgm.Coll(&shared.CoinPendingData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	result := make(map[int][]string)
	for _, v := range list {
		result[v.ShardID] = append(result[v.ShardID], v.TxHash)
	}
	return result, nil
}

func DBGetPendingCoins() ([]string, error) {
	list := []shared.CoinPendingData{}
	filter := bson.M{}
	err := mgm.Coll(&shared.CoinPendingData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, v := range list {
		result = append(result, v.SerialNumber...)
	}
	return result, nil
}

func DBCheckTxsExist(txList []string, shardID int) ([]bool, error) {
	startTime := time.Now()
	var result []bool
	var kmsdata []shared.KeyImageData
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(txList)+1)*shared.DB_OPERATION_TIMEOUT)
	filter := bson.M{"txhash": bson.M{operator.In: txList}, "shardid": bson.M{operator.Eq: shardID}}
	err := mgm.Coll(&shared.KeyImageData{}).SimpleFindWithCtx(ctx, &kmsdata, filter)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	for _, km := range txList {
		found := false
		for _, rkm := range kmsdata {
			if km == rkm.TxHash {
				found = true
				break
			}
		}
		result = append(result, found)
	}
	log.Printf("checked %v keyimages in %v", len(txList), time.Since(startTime))
	return result, nil
}
