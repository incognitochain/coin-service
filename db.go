package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func connectDB() error {
	err := mgm.SetDefaultConfig(nil, "coins", options.Client().ApplyURI(serviceCfg.MongoAddress))
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func DBCreateCoinV1Index() error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
	indexName, err := mgm.Coll(&CoinDataV1{}).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"beaconheight": -1,
		},
		// Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("failed to indexs coins in %v", time.Since(startTime))
		return err
	}
	log.Println("indexName", indexName)
	log.Printf("success indexs coins in %v", time.Since(startTime))
	return nil
}

func DBSaveCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
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
		_, err := mgm.Coll(&CoinData{}).InsertMany(ctx, docs)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docs), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v2coins in %v", len(docs), time.Since(startTime))

	}
	if len(docsV1) > 0 {
		_, err := mgm.Coll(&CoinDataV1{}).InsertMany(ctx, docsV1)
		if err != nil {
			log.Printf("failed to insert %v coins in %v", len(docsV1), time.Since(startTime))
			return err
		}
		log.Printf("inserted %v v1coins in %v", len(docsV1), time.Since(startTime))
	}
	return nil
}

func DBUpdateCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		update := bson.M{
			"$set": coin,
		}
		docs = append(docs, update)
	}
	for idx, doc := range docs {
		fmt.Println(list[idx].GetID())
		_, err := mgm.Coll(&CoinData{}).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v coins in %v", len(list), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v coins in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetCoinsByIndex(idx int, shardID int, tokenID string) (*CoinData, error) {
	var result CoinData
	filter := bson.M{"coinidx": bson.M{operator.Eq: idx}, "shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&CoinData{}).First(filter, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func DBGetCoinsByOTAKey(OTASecret string) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	temp, _, err := base58.DecodeCheck(OTASecret)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"otasecret": bson.M{operator.Eq: hex.EncodeToString(temp)}}
	err = mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinsByOTAKeyAndHeight(tokenID, OTASecret string, fromHeight int, toHeight int) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"otasecret": bson.M{operator.Eq: OTASecret}, "beaconheight": bson.M{operator.Gte: fromHeight, operator.Lte: toHeight}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsFromBeaconHeight(beaconHeight uint64) ([]CoinData, error) {
	limit := int64(500)
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"beaconheight": bson.M{operator.Gte: beaconHeight}, "otasecret": bson.M{operator.Eq: ""}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsFromCoinIndex(prvidx, tokenidx uint64) ([]CoinData, error) {
	limit := int64(500)
	startTime := time.Now()
	listPRV := []CoinData{}
	listToken := []CoinData{}

	filter := bson.M{"coinidx": bson.M{operator.Gte: prvidx}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: common.PRVCoinID.String()}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &listPRV, filter, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v prv coins in %v", len(listPRV), time.Since(startTime))

	startTime = time.Now()
	filterTk := bson.M{"coinidx": bson.M{operator.Gte: tokenidx}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: common.ConfidentialAssetID.String()}}
	ctxTk, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err = mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctxTk, &listToken, filterTk, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v token coins in %v", len(listToken), time.Since(startTime))
	return append(listPRV, listToken...), err
}

func DBGetUnknownCoinsFromCoinIndexWithLimit(index uint64, isPRV bool, limit int64) ([]CoinData, error) {
	tokenID := common.PRVCoinID.String()
	if !isPRV {
		tokenID = common.ConfidentialAssetID.String()
	}
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"coinidx": bson.M{operator.Gte: index}, "otasecret": bson.M{operator.Eq: ""}, "tokenid": bson.M{operator.Eq: tokenID}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Limit: &limit,
	})
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinV1ByPubkey(tokenID, pubkey string, fromHeight int, toHeight int) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"coinpubkey": bson.M{operator.Eq: pubkey}, "beaconheight": bson.M{operator.Gte: fromHeight, operator.Lte: toHeight}, "tokenid": bson.M{operator.Eq: tokenID}}
	err := mgm.Coll(&CoinDataV1{}).SimpleFind(&list, filter)
	if err != nil {
		return nil, err
	}
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetCoinsOTAStat() error {
	return nil
}

func DBSaveUsedKeyimage(list []KeyImageData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		docs = append(docs, coin)
	}
	_, err := mgm.Coll(&list[0]).InsertMany(ctx, docs)
	if err != nil {
		log.Printf("failed to insert %v keyimages in %v", len(list), time.Since(startTime))
		return err
	}
	log.Printf("inserted %v keyimages in %v", len(list), time.Since(startTime))
	return nil
}

func DBCheckKeyimagesUsed(list []string, shardID int) ([]bool, error) {
	startTime := time.Now()
	var result []bool
	for _, keyImage := range list {
		kmBytes, _, err := base58.Base58Check{}.Decode(keyImage)
		if err != nil {
			log.Println(err)
			continue
		}
		var kmdata *CoinData
		filter := bson.M{"keyimage": bson.M{operator.Eq: kmBytes}}
		err = mgm.Coll(&CoinData{}).First(filter, kmdata)
		if err != nil {
			log.Println(err)
			result = append(result, false)
			continue
		}
		result = append(result, true)
	}

	log.Printf("checked %v keyimages in %v", len(list), time.Since(startTime))
	return result, nil
}

func DBGetCoinsOfShardCount(shardID int, tokenID string) int64 {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(5)*DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := CoinData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}
