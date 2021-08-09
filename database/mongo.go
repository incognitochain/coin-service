package database

import (
	"context"
	"log"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/incognito-chain/common"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

func DBSaveCoins(list []shared.CoinData) (error, []shared.CoinDataV1) {
	startTime := time.Now()
	coinV1AlreadyWrite := []shared.CoinDataV1{}
	docs := []interface{}{}
	docsV1 := []interface{}{}
	for _, coin := range list {
		coin.Creating()
		if coin.CoinVersion == 2 {
			docs = append(docs, coin)
		} else {
			docsV1 = append(docsV1, coin)
		}
	}
	if len(docs) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+2)*shared.DB_OPERATION_TIMEOUT)
		_, err := mgm.Coll(&shared.CoinData{}).InsertMany(ctx, docs, options.MergeInsertManyOptions().SetOrdered(true))
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				panic(err)
			}
			if ctx.Err() != nil {
				t, k := ctx.Deadline()
				log.Println("context error:", ctx.Err(), t, k)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			} else {
				for _, v := range docs {
					ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
					_, err = mgm.Coll(&shared.CoinData{}).InsertOne(ctx, v)
					if err != nil {
						writeErr, ok := err.(mongo.WriteException)
						if !ok {
							panic(err)
						}
						if !writeErr.HasErrorCode(11000) {
							panic(err)
						}
					}
				}
			}
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))
	}
	if len(docsV1) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+2)*shared.DB_OPERATION_TIMEOUT)
		_, err := mgm.Coll(&shared.CoinDataV1{}).InsertMany(ctx, docsV1, options.MergeInsertManyOptions().SetOrdered(true))
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				panic(err)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			} else {
				for _, v := range docsV1 {
					ctx, _ := context.WithTimeout(context.Background(), time.Duration(2)*shared.DB_OPERATION_TIMEOUT)
					_, err = mgm.Coll(&shared.CoinDataV1{}).InsertOne(ctx, v)
					if err != nil {
						writeErr, ok := err.(mongo.WriteException)
						if !ok {
							panic(err)
						}
						if !writeErr.HasErrorCode(11000) {
							panic(err)
						}
					}
				}
			}
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil, coinV1AlreadyWrite
}

func DBSavePendingTx(list []shared.CoinPendingData) error {
	if len(list) > 0 {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
		docs := []interface{}{}
		for _, coin := range list {
			coin.Creating()
			docs = append(docs, coin)
		}
		_, err := mgm.Coll(&shared.CoinPendingData{}).InsertMany(ctx, docs)
		if err != nil {
			writeErr, ok := err.(mongo.BulkWriteException)
			if !ok {
				panic(err)
			}
			er := writeErr.WriteErrors[0]
			if er.WriteError.Code != 11000 {
				panic(err)
			} else {
				for _, v := range docs {
					ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*shared.DB_OPERATION_TIMEOUT)
					_, err = mgm.Coll(&shared.CoinPendingData{}).InsertOne(ctx, v)
					if err != nil {
						writeErr, ok := err.(mongo.WriteException)
						if !ok {
							panic(err)
						}
						if !writeErr.HasErrorCode(11000) {
							panic(err)
						}
					}
				}
			}
		}
	}
	return nil
}

func DBDeletePendingTxs(list []string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list)+10)*shared.DB_OPERATION_TIMEOUT)
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
func DBGetPendingTxsData() ([]string, error) {
	list := []shared.CoinPendingData{}
	filter := bson.M{}
	err := mgm.Coll(&shared.CoinPendingData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, v := range list {
		result = append(result, v.TxData)
	}
	return result, nil
}

func DBGetPendingTxDetail(txhash string) (string, error) {
	list := []shared.CoinPendingData{}
	filter := bson.M{"txhash": bson.M{operator.Eq: txhash}}
	err := mgm.Coll(&shared.CoinPendingData{}).SimpleFind(&list, filter, nil)
	if err != nil {
		return "", err
	}
	return list[0].TxData, nil
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
		result = append(result, v.Keyimages...)
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

func DBGetCoinInfo() (map[int]uint64, map[int]uint64, map[int]uint64, map[int]uint64, error) {
	prvV1 := make(map[int]uint64)
	prvV2 := make(map[int]uint64)
	tokenV1 := make(map[int]uint64)
	tokenV2 := make(map[int]uint64)

	for i := 0; i < shared.ServiceCfg.NumOfShard; i++ {

		pv2 := DBGetCoinV2OfShardCount(i, common.PRVCoinID.String())

		tkv2 := DBGetCoinV2OfShardCount(i, common.ConfidentialAssetID.String())

		pv1 := DBGetCoinV1OfShardCount(i, common.PRVCoinID.String())

		tkv1 := DBGetCoinTokenV1OfShardCount(i)

		prvV1[i] = uint64(pv1)
		prvV2[i] = uint64(pv2)
		tokenV1[i] = uint64(tkv1)
		tokenV2[i] = uint64(tkv2)
	}

	return prvV1, prvV2, tokenV1, tokenV2, nil
}
