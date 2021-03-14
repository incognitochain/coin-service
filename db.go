package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/operator"
	"go.mongodb.org/mongo-driver/bson"
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

func DBSaveCoins(list []CoinData) error {
	startTime := time.Now()
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(len(list))*DB_OPERATION_TIMEOUT)
	docs := []interface{}{}
	for _, coin := range list {
		docs = append(docs, coin)
	}
	_, err := mgm.Coll(&list[0]).InsertMany(ctx, docs)
	if err != nil {
		log.Printf("failed to insert %v coins in %v", len(list), time.Since(startTime))
		return err
	}
	log.Printf("inserted %v coins in %v", len(list), time.Since(startTime))
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
		_, err := mgm.Coll(&list[0]).UpdateByID(ctx, list[idx].GetID(), doc)
		if err != nil {
			log.Printf("failed to update %v coins in %v", len(list), time.Since(startTime))
			return err
		}
	}
	log.Printf("updated %v coins in %v", len(list), time.Since(startTime))
	return nil
}

func DBGetCoinsByOTAKey(OTASecret string) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"otasecret": bson.M{operator.Eq: OTASecret}}
	err := mgm.Coll(&CoinData{}).SimpleFind(&list, filter)
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
	log.Printf("found %v coins in %v", len(list), time.Since(startTime))
	return list, err
}

func DBGetUnknownCoinsFromIndex(index int, limit int64) ([]CoinData, error) {
	startTime := time.Now()
	list := []CoinData{}
	filter := bson.M{"coinidx": bson.M{operator.Gt: index}, "otasecret": bson.M{operator.Eq: ""}}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(limit)*DB_OPERATION_TIMEOUT)
	err := mgm.Coll(&CoinData{}).SimpleFindWithCtx(ctx, &list, filter, &options.FindOptions{
		Limit: &limit,
	})
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
		kmBytes, err := hex.DecodeString(keyImage)
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
	ctx, _ := context.WithTimeout(context.Background(), 5*DB_OPERATION_TIMEOUT)
	filter := bson.M{"shardid": bson.M{operator.Eq: shardID}, "tokenid": bson.M{operator.Eq: tokenID}}
	doc := KeyImageData{}
	count, err := mgm.Coll(&doc).CountDocuments(ctx, filter)
	if err != nil {
		log.Println(err)
		return -1
	}
	return count
}
